package handlers

import (
	"LabPanel/middleware"
	"LabPanel/models"
	"LabPanel/service"
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func TestPortForwardAPI(t *testing.T) {
	t.Setenv("JWT_SECRET", "port-forward-api-test")
	t.Setenv("ADMIN_HASHED_PASSWORD", "unused-test-hash")
	if err := service.InitPortForwardService(filepath.Join(t.TempDir(), "forwards.json")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(service.GetPortForwardService().Close)
	router := gin.New()
	auth := router.Group("/api", middleware.AuthMiddleware())
	auth.GET("/port-forwards", GetPortForwards)
	auth.POST("/port-forwards", CreatePortForward)
	auth.PUT("/port-forwards/:id", UpdatePortForward)
	auth.DELETE("/port-forwards/:id", DeletePortForward)
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"exp": time.Now().Add(time.Hour).Unix(),
	}).SignedString([]byte("port-forward-api-test"))
	if err != nil {
		t.Fatal(err)
	}
	request := func(method, path string, body interface{}, authenticated bool, want int) *httptest.ResponseRecorder {
		t.Helper()
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(method, "/api/port-forwards"+path, bytes.NewReader(data))
		req.Header.Set("Content-Type", "application/json")
		if authenticated {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		if response.Code != want {
			t.Fatalf("%s %s: status %d, want %d: %s", method, path, response.Code, want, response.Body.String())
		}
		return response
	}
	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete} {
		path := ""
		if method == http.MethodPut || method == http.MethodDelete {
			path = "/unknown"
		}
		request(method, path, nil, false, http.StatusUnauthorized)
	}
	// A failed bind must return a conflict without persisting a partial rule.
	occupied, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer occupied.Close()
	rule := models.PortForward{
		Name: "test web", Protocol: "tcp", Enabled: true,
		ListenAddress: "127.0.0.1", ListenPort: occupied.Addr().(*net.TCPAddr).Port,
		TargetAddress: "192.0.2.1", TargetPort: 8080,
	}
	request(http.MethodPost, "", rule, true, http.StatusConflict)
	if len(service.GetPortForwardService().List()) != 0 {
		t.Fatal("failed create persisted a rule")
	}
	rule.Enabled = false
	response := request(http.MethodPost, "", rule, true, http.StatusCreated)
	var created struct {
		Forward models.PortForwardStatus `json:"forward"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Forward.ID == "" || created.Forward.Status != "stopped" {
		t.Fatalf("unexpected created forward: %+v", created.Forward)
	}
	path := "/" + created.Forward.ID
	response = request(http.MethodGet, "", nil, true, http.StatusOK)
	var listed struct {
		Forwards []models.PortForwardStatus `json:"forwards"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &listed); err != nil || len(listed.Forwards) != 1 {
		t.Fatalf("unexpected list: %s (error %v)", response.Body.String(), err)
	}
	invalid := rule
	invalid.TargetPort = 0
	request(http.MethodPut, path, invalid, true, http.StatusBadRequest)
	request(http.MethodPut, "/missing", rule, true, http.StatusNotFound)
	rule.Name = "renamed web"
	request(http.MethodPut, path, rule, true, http.StatusOK)
	if service.GetPortForwardService().List()[0].Name != rule.Name {
		t.Fatal("update did not reach service")
	}
	request(http.MethodDelete, path, nil, true, http.StatusOK)
	request(http.MethodDelete, path, nil, true, http.StatusNotFound)
	if len(service.GetPortForwardService().List()) != 0 {
		t.Fatal("delete left a rule in the service")
	}
}
