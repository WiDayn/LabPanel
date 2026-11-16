package service

import (
	"LabPanel/models"
	"bytes"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"
)

func runtimeTestRule(t *testing.T, protocol, target string) models.PortForward {
	t.Helper()
	host, port, err := net.SplitHostPort(target)
	if err != nil {
		t.Fatal(err)
	}
	parsedPort, err := strconv.Atoi(port)
	if err != nil {
		t.Fatal(err)
	}
	return models.PortForward{
		Protocol: protocol, ListenAddress: "127.0.0.1", ListenPort: 0,
		TargetAddress: host, TargetPort: parsedPort, Enabled: true,
	}
}

// The fixture closes every accepted socket so failures never leave workers alive.
func runtimeTestTCPServer(t *testing.T, address string, handler func(net.Conn)) net.Listener {
	t.Helper()
	listener, err := net.Listen("tcp", address)
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	connections := make(map[net.Conn]bool)
	accepted := make(chan struct{})
	var workers sync.WaitGroup
	workers.Add(1)
	go func() {
		defer workers.Done()
		defer close(accepted)
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			mu.Lock()
			connections[conn] = true
			mu.Unlock()
			workers.Add(1)
			go func() {
				defer workers.Done()
				defer conn.Close()
				defer func() {
					mu.Lock()
					delete(connections, conn)
					mu.Unlock()
				}()
				handler(conn)
			}()
		}
	}()
	t.Cleanup(func() {
		_ = listener.Close()
		// Accept may already have returned another socket before Close. Wait for
		// the accept loop before taking the final socket snapshot.
		<-accepted
		mu.Lock()
		for conn := range connections {
			_ = conn.Close()
		}
		mu.Unlock()
		workers.Wait()
	})
	return listener
}

func runtimeTestEcho(prefix string) func(net.Conn) {
	return func(conn net.Conn) {
		buffer := make([]byte, 4096)
		for {
			n, err := conn.Read(buffer)
			if err != nil {
				return
			}
			if _, err := conn.Write(append([]byte(prefix), buffer[:n]...)); err != nil {
				return
			}
		}
	}
}

func runtimeTestTCPExchange(t *testing.T, conn net.Conn, input, expected string) {
	t.Helper()
	if err := conn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(conn, input); err != nil {
		t.Fatal(err)
	}
	response := make([]byte, len(expected))
	if _, err := io.ReadFull(conn, response); err != nil {
		t.Fatal(err)
	}
	if string(response) != expected {
		t.Fatalf("unexpected response %q, want %q", response, expected)
	}
}

func TestPortForwardRuntimeTCPHalfCloseAndRelease(t *testing.T) {
	upstream := runtimeTestTCPServer(t, "127.0.0.1:0", func(conn net.Conn) {
		request, err := io.ReadAll(conn)
		if err == nil {
			_, _ = conn.Write(append([]byte("response:"), request...))
		}
	})
	runtime, err := startPortForward(runtimeTestRule(t, "tcp", upstream.Addr().String()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(runtime.Close)
	address := runtime.tcpListener.Addr().String()
	conn, err := net.DialTimeout("tcp", address, 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
	payload := bytes.Repeat([]byte("large request\x00"), 8192)
	if _, err := conn.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := conn.(*net.TCPConn).CloseWrite(); err != nil {
		t.Fatal(err)
	}
	response, err := io.ReadAll(conn)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(response, append([]byte("response:"), payload...)) {
		t.Fatal("half-closed request did not receive its complete response")
	}
	runtime.Close()
	if err := runtime.Err(); err != nil {
		t.Fatalf("normal shutdown reported a failure: %v", err)
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		t.Fatalf("listener was not released: %v", err)
	}
	_ = listener.Close()
}

func TestPortForwardRuntimeTCPUpdateAndStopActiveConnections(t *testing.T) {
	upstreamA := runtimeTestTCPServer(t, "127.0.0.1:0", runtimeTestEcho("A:"))
	upstreamB := runtimeTestTCPServer(t, "127.0.0.1:0", runtimeTestEcho("B:"))
	rule := runtimeTestRule(t, "tcp", upstreamA.Addr().String())
	runtime, err := startPortForward(rule)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(runtime.Close)
	address := runtime.tcpListener.Addr().String()
	oldConn, err := net.DialTimeout("tcp", address, 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer oldConn.Close()
	runtimeTestTCPExchange(t, oldConn, "first", "A:first")
	newTarget := runtimeTestRule(t, "tcp", upstreamB.Addr().String())
	runtime.SetTarget(newTarget.TargetAddress, newTarget.TargetPort)
	runtimeTestTCPExchange(t, oldConn, "existing", "A:existing")
	newConn, err := net.DialTimeout("tcp", address, 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer newConn.Close()
	runtimeTestTCPExchange(t, newConn, "new", "B:new")
	done := make(chan struct{})
	go func() { runtime.Close(); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("stop did not terminate active connections")
	}
	for _, conn := range []net.Conn{oldConn, newConn} {
		_ = conn.SetReadDeadline(time.Now().Add(time.Second))
		if _, err := conn.Read(make([]byte, 1)); err == nil {
			t.Fatal("active connection remained open after stop")
		} else if timeout, ok := err.(net.Error); ok && timeout.Timeout() {
			t.Fatal("active connection was not closed")
		}
	}
}

func runtimeTestUDPServer(t *testing.T, prefix string) net.PacketConn {
	t.Helper()
	conn, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		buffer := make([]byte, 65535)
		for {
			n, client, err := conn.ReadFrom(buffer)
			if err != nil {
				return
			}
			_, _ = conn.WriteTo(append([]byte(prefix), buffer[:n]...), client)
		}
	}()
	t.Cleanup(func() { _ = conn.Close(); <-done })
	return conn
}

func runtimeTestUDPExchange(t *testing.T, conn net.Conn, input, expected string) {
	t.Helper()
	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
	if _, err := conn.Write([]byte(input)); err != nil {
		t.Fatal(err)
	}
	response := make([]byte, 65535)
	n, err := conn.Read(response)
	if err != nil {
		t.Fatal(err)
	}
	if string(response[:n]) != expected {
		t.Fatalf("unexpected datagram %q, want %q", response[:n], expected)
	}
}

func TestPortForwardRuntimeUDPMultipleClientsAndTargetUpdate(t *testing.T) {
	upstreamA := runtimeTestUDPServer(t, "A:")
	upstreamB := runtimeTestUDPServer(t, "B:")
	runtime, err := startPortForward(runtimeTestRule(t, "udp", upstreamA.LocalAddr().String()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(runtime.Close)
	address := runtime.udpListener.LocalAddr().String()
	var clients []net.Conn
	for i := 0; i < 2; i++ {
		client, err := net.Dial("udp", address)
		if err != nil {
			t.Fatal(err)
		}
		defer client.Close()
		clients = append(clients, client)
	}
	for i := 0; i < 3; i++ {
		for j, client := range clients {
			input := fmt.Sprintf("client-%d-message-%d", j, i)
			runtimeTestUDPExchange(t, client, input, "A:"+input)
		}
	}
	newTarget := runtimeTestRule(t, "udp", upstreamB.LocalAddr().String())
	runtime.SetTarget(newTarget.TargetAddress, newTarget.TargetPort)
	for _, client := range clients {
		runtimeTestUDPExchange(t, client, "changed", "B:changed")
	}
	runtime.Close()
	conn, err := net.ListenPacket("udp", address)
	if err != nil {
		t.Fatalf("UDP listener was not released: %v", err)
	}
	_ = conn.Close()
	if err := runtime.Err(); err != nil {
		t.Fatalf("normal UDP shutdown reported a failure: %v", err)
	}
}

func TestPortForwardRuntimeUDPIdleExpiry(t *testing.T) {
	upstream := runtimeTestUDPServer(t, "")
	runtime, err := startPortForward(runtimeTestRule(t, "udp", upstream.LocalAddr().String()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(runtime.Close)
	runtime.mu.Lock()
	runtime.udpIdle = 30 * time.Millisecond
	runtime.mu.Unlock()
	client, err := net.Dial("udp", runtime.udpListener.LocalAddr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	runtimeTestUDPExchange(t, client, "first", "first")
	deadline := time.Now().Add(3 * time.Second)
	for {
		runtime.mu.Lock()
		remaining := len(runtime.sessions)
		runtime.mu.Unlock()
		if remaining == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("idle UDP session was not cleaned up")
		}
		time.Sleep(5 * time.Millisecond)
	}
	runtimeTestUDPExchange(t, client, "recreated", "recreated")
}

func TestPortForwardRuntimeUnexpectedListenerFailure(t *testing.T) {
	runtime, err := startPortForward(models.PortForward{
		Protocol: "tcp", ListenAddress: "127.0.0.1", TargetAddress: "127.0.0.1", TargetPort: 9,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(runtime.Close)
	if err := runtime.tcpListener.Close(); err != nil {
		t.Fatal(err)
	}
	finished := make(chan struct{})
	go func() { runtime.wg.Wait(); close(finished) }()
	select {
	case <-finished:
	case <-time.After(2 * time.Second):
		t.Fatal("runtime did not stop after its listener failed")
	}
	if runtime.Err() == nil {
		t.Fatal("unexpected listener failure was not reported")
	}
}

func TestPortForwardRuntimeBindFailure(t *testing.T) {
	for _, protocol := range []string{"tcp", "udp"} {
		t.Run(protocol, func(t *testing.T) {
			rule := runtimeTestRule(t, protocol, "127.0.0.1:12345")
			first, err := startPortForward(rule)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(first.Close)
			var address string
			if protocol == "tcp" {
				address = first.tcpListener.Addr().String()
			} else {
				address = first.udpListener.LocalAddr().String()
			}
			_, port, _ := net.SplitHostPort(address)
			rule.ListenPort, _ = strconv.Atoi(port)
			if second, err := startPortForward(rule); err == nil {
				second.Close()
				t.Fatal("occupied listener did not fail synchronously")
			}
		})
	}
}

func TestPortForwardRuntimeIPv6(t *testing.T) {
	probe, err := net.Listen("tcp6", "[::1]:0")
	if err != nil {
		t.Skipf("IPv6 loopback unavailable: %v", err)
	}
	_ = probe.Close()
	upstream := runtimeTestTCPServer(t, "[::1]:0", runtimeTestEcho("v6:"))
	rule := runtimeTestRule(t, "tcp", upstream.Addr().String())
	rule.ListenAddress = "::1"
	runtime, err := startPortForward(rule)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(runtime.Close)
	conn, err := net.DialTimeout("tcp6", runtime.tcpListener.Addr().String(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	runtimeTestTCPExchange(t, conn, "message", "v6:message")
}
