package service

import (
	"LabPanel/models"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestPortForwardServicePersistsAndRestoresListeners(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "forwards.json")
	s, err := NewPortForwardService(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	if got := s.List(); got == nil || len(got) != 0 {
		t.Fatalf("new store should return an empty array, got %#v", got)
	}

	targetPort := portForwardServiceEcho(t)
	rule := portForwardServiceRule(t, "web", targetPort)
	created, err := s.Save("", rule)
	if err != nil {
		t.Fatal(err)
	}
	if created.ID == "" || created.Status != "running" {
		t.Fatalf("unexpected creation result: %+v", created)
	}
	portForwardServiceRoundTrip(t, created.ListenPort)

	disabled := portForwardServiceRule(t, "disabled", targetPort)
	disabled.Enabled = false
	disabledStatus, err := s.Save("", disabled)
	if err != nil {
		t.Fatal(err)
	}
	if disabledStatus.Status != "stopped" {
		t.Fatalf("disabled rule is not stopped: %+v", disabledStatus)
	}
	portForwardServiceAssertPortFree(t, disabled.ListenPort)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var stored portForwardStore
	if err := json.Unmarshal(data, &stored); err != nil || len(stored.Forwards) != 2 {
		t.Fatalf("unexpected saved store %s: %v", data, err)
	}
	if strings.Contains(string(data), `"status"`) || strings.Contains(string(data), `"error"`) {
		t.Fatalf("runtime state should not be persisted: %s", data)
	}

	s.Close()
	restored, err := NewPortForwardService(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(restored.Close)
	states := restored.List()
	if len(states) != 2 || states[0].ID != created.ID || states[0].Status != "running" || states[1].Status != "stopped" {
		t.Fatalf("unexpected restored states: %+v", states)
	}
	portForwardServiceRoundTrip(t, created.ListenPort)
	portForwardServiceAssertPortFree(t, disabled.ListenPort)

	if err := restored.Delete(created.ID); err != nil {
		t.Fatal(err)
	}
	portForwardServiceAssertPortFree(t, created.ListenPort)
	if got := restored.List(); len(got) != 1 || got[0].ID != disabledStatus.ID {
		t.Fatalf("delete removed the wrong rule: %+v", got)
	}
	if err := restored.Delete(created.ID); !errors.Is(err, ErrPortForwardNotFound) {
		t.Fatalf("expected missing rule error, got %v", err)
	}
	if _, err := restored.Save("missing", rule); !errors.Is(err, ErrPortForwardNotFound) {
		t.Fatalf("expected missing rule on update, got %v", err)
	}
}

func TestPortForwardServiceConflictPreservesWorkingRule(t *testing.T) {
	s := portForwardServiceNew(t)
	rule := portForwardServiceRule(t, "web", portForwardServiceEcho(t))
	created, err := s.Save("", rule)
	if err != nil {
		t.Fatal(err)
	}

	duplicateName := rule
	duplicateName.Name = "WEB"
	duplicateName.Enabled = false
	if _, err := s.Save("", duplicateName); !errors.Is(err, ErrPortForwardConflict) {
		t.Fatalf("duplicate name should conflict: %v", err)
	}
	duplicatePort := rule
	duplicatePort.Name = "other"
	if _, err := s.Save("", duplicatePort); !errors.Is(err, ErrPortForwardConflict) {
		t.Fatalf("duplicate listener should conflict: %v", err)
	}

	blocker, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = blocker.Close() })
	updated := rule
	updated.ListenPort = blocker.Addr().(*net.TCPAddr).Port
	if _, err := s.Save(created.ID, updated); !errors.Is(err, ErrPortForwardConflict) {
		t.Fatalf("occupied update should conflict: %v", err)
	}
	portForwardServiceRoundTrip(t, created.ListenPort)
	if got := s.List(); len(got) != 1 || got[0].ListenPort != created.ListenPort || got[0].Status != "running" {
		t.Fatalf("conflict changed the original rule: %+v", got)
	}

	// TCP and UDP are separate host listeners even when their port numbers match.
	udp := rule
	udp.Name, udp.Protocol = "udp", "udp"
	if _, err := s.Save("", udp); err != nil {
		t.Fatalf("TCP and UDP should share a port number: %v", err)
	}
}

func TestPortForwardServiceFailedPersistenceRollsBack(t *testing.T) {
	s := portForwardServiceNew(t)
	rule := portForwardServiceRule(t, "web", portForwardServiceEcho(t))
	created, err := s.Save("", rule)
	if err != nil {
		t.Fatal(err)
	}
	originalPath := s.path
	originalData, err := os.ReadFile(originalPath)
	if err != nil {
		t.Fatal(err)
	}
	// Renaming a temporary file over a directory fails even when tests run as root.
	s.path = filepath.Join(t.TempDir(), "not-a-file")
	if err := os.Mkdir(s.path, 0755); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		edit func(*models.PortForward)
	}{
		{"destination", func(r *models.PortForward) { r.TargetPort = 1 }},
		{"different port", func(r *models.PortForward) { r.ListenPort = portForwardServiceFreePort(t) }},
		{"overlapping address", func(r *models.PortForward) { r.ListenAddress = "0.0.0.0" }},
		{"disable", func(r *models.PortForward) { r.Enabled = false }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			updated := rule
			test.edit(&updated)
			if _, err := s.Save(created.ID, updated); err == nil {
				t.Fatal("save should fail")
			}
			portForwardServiceRoundTrip(t, created.ListenPort)
			if got := s.List(); len(got) != 1 || got[0].PortForward != created.PortForward || got[0].Status != "running" {
				t.Fatalf("failed persistence changed active rule: %+v", got)
			}
			if updated.ListenPort != rule.ListenPort {
				portForwardServiceAssertPortFree(t, updated.ListenPort)
			}
		})
	}
	if err := s.Delete(created.ID); err == nil {
		t.Fatal("delete should fail when persistence fails")
	}
	portForwardServiceRoundTrip(t, created.ListenPort)
	data, err := os.ReadFile(originalPath)
	if err != nil || string(data) != string(originalData) {
		t.Fatalf("failed mutations changed the saved configuration: %v, %s", err, data)
	}
}

func TestPortForwardServiceRestoresOtherRulesWhenOnePortIsOccupied(t *testing.T) {
	path := filepath.Join(t.TempDir(), "forwards.json")
	s, err := NewPortForwardService(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	first := portForwardServiceRule(t, "blocked", 9)
	firstStatus, err := s.Save("", first)
	if err != nil {
		t.Fatal(err)
	}
	second := portForwardServiceRule(t, "working", portForwardServiceEcho(t))
	secondStatus, err := s.Save("", second)
	if err != nil {
		t.Fatal(err)
	}
	s.Close()

	blocker, err := net.Listen("tcp4", net.JoinHostPort("127.0.0.1", strconv.Itoa(first.ListenPort)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = blocker.Close() })
	restored, err := NewPortForwardService(path)
	if err != nil {
		t.Fatalf("a port conflict should not prevent loading other rules: %v", err)
	}
	t.Cleanup(restored.Close)
	states := restored.List()
	if len(states) != 2 || states[0].Status != "error" || states[0].Error == "" || !states[0].Enabled || states[1].Status != "running" {
		t.Fatalf("unexpected startup states: %+v", states)
	}
	portForwardServiceRoundTrip(t, secondStatus.ListenPort)
	_ = blocker.Close()
	retried, err := restored.Save(firstStatus.ID, first)
	if err != nil || retried.Status != "running" {
		t.Fatalf("failed rule should be recoverable: %+v, %v", retried, err)
	}
}

func TestPortForwardServiceReinitializationReleasesPreviousListeners(t *testing.T) {
	path := filepath.Join(t.TempDir(), "forwards.json")
	if err := InitPortForwardService(path); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		portForwardServiceMu.Lock()
		defer portForwardServiceMu.Unlock()
		if defaultPortForwardService != nil {
			defaultPortForwardService.Close()
			defaultPortForwardService = nil
		}
	})
	previous := GetPortForwardService()
	rule := portForwardServiceRule(t, "web", portForwardServiceEcho(t))
	created, err := previous.Save("", rule)
	if err != nil {
		t.Fatal(err)
	}
	if err := InitPortForwardService(path); err != nil {
		t.Fatal(err)
	}
	reinitialized := GetPortForwardService()
	if reinitialized == previous {
		t.Fatal("initialization did not replace the service")
	}
	if states := reinitialized.List(); len(states) != 1 || states[0].Status != "running" {
		t.Fatalf("reinitialization conflicted with previous listeners: %+v", states)
	}
	portForwardServiceRoundTrip(t, created.ListenPort)
}

func TestPortForwardServiceRejectsInvalidRules(t *testing.T) {
	s := portForwardServiceNew(t)
	base := models.PortForward{
		Name: "test", Protocol: "tcp", ListenAddress: "127.0.0.1", ListenPort: 18080,
		TargetAddress: "example.com", TargetPort: 8080, Enabled: false,
	}
	tests := []struct {
		name string
		edit func(*models.PortForward)
	}{
		{"empty name", func(r *models.PortForward) { r.Name = "  " }},
		{"protocol", func(r *models.PortForward) { r.Protocol = "http" }},
		{"listen hostname", func(r *models.PortForward) { r.ListenAddress = "localhost" }},
		{"listen URL", func(r *models.PortForward) { r.ListenAddress = "http://127.0.0.1" }},
		{"listen port zero", func(r *models.PortForward) { r.ListenPort = 0 }},
		{"listen port too large", func(r *models.PortForward) { r.ListenPort = 65536 }},
		{"target port negative", func(r *models.PortForward) { r.TargetPort = -1 }},
		{"target port too large", func(r *models.PortForward) { r.TargetPort = 65536 }},
		{"empty target", func(r *models.PortForward) { r.TargetAddress = "" }},
		{"target URL", func(r *models.PortForward) { r.TargetAddress = "https://example.com" }},
		{"target port in host", func(r *models.PortForward) { r.TargetAddress = "example.com:8080" }},
		{"target path", func(r *models.PortForward) { r.TargetAddress = "example.com/path" }},
		{"target whitespace", func(r *models.PortForward) { r.TargetAddress = "example .com" }},
		{"unspecified target IPv4", func(r *models.PortForward) { r.TargetAddress = "0.0.0.0" }},
		{"unspecified target IPv6", func(r *models.PortForward) { r.TargetAddress = "::" }},
		{"self loop", func(r *models.PortForward) { r.TargetAddress, r.TargetPort = r.ListenAddress, r.ListenPort }},
		{"wildcard self loop", func(r *models.PortForward) {
			r.ListenAddress, r.TargetAddress, r.TargetPort = "0.0.0.0", "127.0.0.1", r.ListenPort
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rule := base
			test.edit(&rule)
			if _, err := s.Save("", rule); !errors.Is(err, ErrPortForwardInvalid) {
				t.Fatalf("expected invalid rule error, got %v", err)
			}
		})
	}
	if got := s.List(); len(got) != 0 {
		t.Fatalf("invalid rules were saved: %+v", got)
	}

	base.Name, base.Protocol, base.ListenAddress, base.TargetAddress = "  ipv6  ", " TCP ", "[::1]", "[2001:db8::1]"
	valid, err := s.Save("", base)
	if err != nil {
		t.Fatalf("valid IPv6 rule rejected: %v", err)
	}
	if valid.Name != "ipv6" || valid.Protocol != "tcp" || valid.ListenAddress != "::1" || valid.TargetAddress != "2001:db8::1" {
		t.Fatalf("rule not normalized: %+v", valid)
	}
}

func TestPortForwardServiceDoesNotOverwriteCorruptStore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "forwards.json")
	contents := []byte(`{"forwards":[`)
	if err := os.WriteFile(path, contents, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewPortForwardService(path); err == nil {
		t.Fatal("corrupt configuration should fail to load")
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != string(contents) {
		t.Fatalf("failed load overwrote configuration: %q, %v", got, err)
	}
}

func portForwardServiceNew(t *testing.T) *PortForwardService {
	t.Helper()
	s, err := NewPortForwardService(filepath.Join(t.TempDir(), "forwards.json"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return s
}

func portForwardServiceRule(t *testing.T, name string, targetPort int) models.PortForward {
	t.Helper()
	return models.PortForward{
		Name: name, Protocol: "tcp", ListenAddress: "127.0.0.1", ListenPort: portForwardServiceFreePort(t),
		TargetAddress: "127.0.0.1", TargetPort: targetPort, Enabled: true, Comment: "test rule",
	}
}

func portForwardServiceFreePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()
	return port
}

func portForwardServiceAssertPortFree(t *testing.T, port int) {
	t.Helper()
	listener, err := net.Listen("tcp4", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		t.Fatalf("port %d was not released: %v", port, err)
	}
	_ = listener.Close()
}

func portForwardServiceEcho(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
				_, _ = io.Copy(conn, conn)
			}()
		}
	}()
	return listener.Addr().(*net.TCPAddr).Port
}

func portForwardServiceRoundTrip(t *testing.T, port int) {
	t.Helper()
	conn, err := net.DialTimeout("tcp4", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), 2*time.Second)
	if err != nil {
		t.Fatalf("listener unavailable: %v", err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	payload := "forwarded through host port"
	if _, err := io.WriteString(conn, payload); err != nil {
		t.Fatal(err)
	}
	response := make([]byte, len(payload))
	if _, err := io.ReadFull(conn, response); err != nil || string(response) != payload {
		t.Fatalf("unexpected forwarded response %q: %v", response, err)
	}
}
