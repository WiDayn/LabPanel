package service

import (
	"LabPanel/models"
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

const (
	forwardDialTimeout = 10 * time.Second
	forwardUDPIdleTime = 2 * time.Minute
	maxForwardSessions = 1024
	maxQueuedDatagrams = 8
)

// portForwardRuntime owns a single host listener and all connections using it.
// It deliberately has no dependency on FRP, container runtimes, or firewall tools.
type portForwardRuntime struct {
	ctx    context.Context
	cancel context.CancelFunc

	tcpListener net.Listener
	udpListener net.PacketConn

	mu       sync.Mutex
	target   string
	closed   bool
	err      error
	tcpConns map[net.Conn]struct{}
	sessions map[string]*forwardUDPSession
	udpIdle  time.Duration
	wg       sync.WaitGroup
}

type forwardUDPSession struct {
	ctx      context.Context
	cancel   context.CancelFunc
	client   net.Addr
	packets  chan []byte
	activity atomic.Int64
	mu       sync.Mutex
	conn     net.Conn
	closed   bool
}

func startPortForward(rule models.PortForward) (*portForwardRuntime, error) {
	ctx, cancel := context.WithCancel(context.Background())
	runtime := &portForwardRuntime{
		ctx: ctx, cancel: cancel,
		target:   net.JoinHostPort(rule.TargetAddress, strconv.Itoa(rule.TargetPort)),
		tcpConns: make(map[net.Conn]struct{}),
		sessions: make(map[string]*forwardUDPSession),
		udpIdle:  forwardUDPIdleTime,
	}
	address := net.JoinHostPort(rule.ListenAddress, strconv.Itoa(rule.ListenPort))
	network := rule.Protocol
	if ip := net.ParseIP(rule.ListenAddress); ip != nil {
		if ip.To4() != nil {
			network += "4"
		} else {
			network += "6"
		}
	}
	var err error
	switch rule.Protocol {
	case "tcp":
		runtime.tcpListener, err = net.Listen(network, address)
	case "udp":
		runtime.udpListener, err = net.ListenPacket(network, address)
	default:
		err = fmt.Errorf("不支持的转发协议 %q", rule.Protocol)
	}
	if err != nil {
		cancel()
		return nil, err
	}
	runtime.wg.Add(1)
	if runtime.tcpListener != nil {
		go runtime.acceptTCP()
	} else {
		go runtime.receiveUDP()
	}
	return runtime, nil
}

// SetTarget keeps the host port bound. Existing TCP streams finish against their
// original destination; UDP sessions are cleared so new packets use the new one.
func (r *portForwardRuntime) SetTarget(address string, port int) {
	target := net.JoinHostPort(address, strconv.Itoa(port))
	r.mu.Lock()
	if r.target == target || r.closed {
		r.mu.Unlock()
		return
	}
	r.target = target
	sessions := r.sessions
	r.sessions = make(map[string]*forwardUDPSession)
	r.mu.Unlock()
	for _, session := range sessions {
		session.stop()
	}
}

// Err reports a fatal listener failure, rather than individual unreachable peers.
func (r *portForwardRuntime) Err() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.err
}

func (r *portForwardRuntime) Close() {
	r.shutdown(nil)
	r.wg.Wait()
}

func (r *portForwardRuntime) shutdown(err error) {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return
	}
	r.closed = true
	r.err = err
	r.cancel()
	connections := make([]net.Conn, 0, len(r.tcpConns))
	for conn := range r.tcpConns {
		connections = append(connections, conn)
	}
	sessions := r.sessions
	r.sessions = make(map[string]*forwardUDPSession)
	r.mu.Unlock()
	if r.tcpListener != nil {
		_ = r.tcpListener.Close()
	}
	if r.udpListener != nil {
		_ = r.udpListener.Close()
	}
	for _, conn := range connections {
		_ = conn.Close()
	}
	for _, session := range sessions {
		session.stop()
	}
}

func (r *portForwardRuntime) acceptTCP() {
	defer r.wg.Done()
	for {
		conn, err := r.tcpListener.Accept()
		if err != nil {
			r.shutdown(err)
			return
		}
		r.mu.Lock()
		// Count accepted and upstream sockets together to bound active resources.
		if r.closed || len(r.tcpConns) >= 2*maxForwardSessions {
			r.mu.Unlock()
			_ = conn.Close()
			continue
		}
		r.tcpConns[conn] = struct{}{}
		target := r.target
		r.mu.Unlock()
		r.wg.Add(1)
		go r.forwardTCP(conn, target)
	}
}

func (r *portForwardRuntime) forwardTCP(client net.Conn, target string) {
	defer r.wg.Done()
	defer r.removeTCPConn(client)
	dialer := net.Dialer{Timeout: forwardDialTimeout, KeepAlive: 30 * time.Second}
	upstream, err := dialer.DialContext(r.ctx, "tcp", target)
	if err != nil {
		return
	}
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		_ = upstream.Close()
		return
	}
	r.tcpConns[upstream] = struct{}{}
	r.mu.Unlock()
	defer r.removeTCPConn(upstream)

	// Wait for both directions: a client may half-close its request and still
	// expect a response (for example, protocols that use EOF as framing).
	done := make(chan struct{})
	go func() {
		copyForwardTCP(upstream, client)
		close(done)
	}()
	copyForwardTCP(client, upstream)
	<-done
}

func copyForwardTCP(dst, src net.Conn) {
	if _, err := io.Copy(dst, src); err != nil {
		_ = dst.Close()
		_ = src.Close()
		return
	}
	if tcp, ok := dst.(*net.TCPConn); ok {
		_ = tcp.CloseWrite()
	} else {
		_ = dst.Close()
	}
}

func (r *portForwardRuntime) removeTCPConn(conn net.Conn) {
	_ = conn.Close()
	r.mu.Lock()
	delete(r.tcpConns, conn)
	r.mu.Unlock()
}

func (r *portForwardRuntime) receiveUDP() {
	defer r.wg.Done()
	buffer := make([]byte, 65535)
	for {
		n, client, err := r.udpListener.ReadFrom(buffer)
		if err != nil {
			r.shutdown(err)
			return
		}
		key := client.String()
		r.mu.Lock()
		if r.closed {
			r.mu.Unlock()
			return
		}
		session := r.sessions[key]
		if session == nil {
			if len(r.sessions) >= maxForwardSessions {
				r.mu.Unlock()
				continue
			}
			ctx, cancel := context.WithCancel(r.ctx)
			session = &forwardUDPSession{
				ctx: ctx, cancel: cancel, client: client,
				packets: make(chan []byte, maxQueuedDatagrams),
			}
			session.activity.Store(time.Now().UnixNano())
			r.sessions[key] = session
			target := r.target
			r.wg.Add(1)
			go r.forwardUDP(key, session, target)
		}
		r.mu.Unlock()
		packet := append([]byte(nil), buffer[:n]...)
		select {
		case session.packets <- packet:
		case <-session.ctx.Done():
		default: // UDP packets can be dropped when a peer cannot keep up.
		}
	}
}

func (r *portForwardRuntime) forwardUDP(key string, session *forwardUDPSession, target string) {
	defer r.wg.Done()
	defer func() {
		session.stop()
		r.mu.Lock()
		if r.sessions[key] == session {
			delete(r.sessions, key)
		}
		r.mu.Unlock()
	}()
	dialer := net.Dialer{Timeout: forwardDialTimeout}
	conn, err := dialer.DialContext(session.ctx, "udp", target)
	if err != nil {
		return
	}
	session.mu.Lock()
	if session.closed {
		session.mu.Unlock()
		_ = conn.Close()
		return
	}
	session.conn = conn
	session.mu.Unlock()
	r.wg.Add(1)
	go r.receiveUDPReplies(session, conn)
	for {
		select {
		case packet := <-session.packets:
			_ = conn.SetWriteDeadline(time.Now().Add(forwardDialTimeout))
			if _, err := conn.Write(packet); err != nil {
				return
			}
			session.activity.Store(time.Now().UnixNano())
		case <-session.ctx.Done():
			return
		}
	}
}

func (r *portForwardRuntime) receiveUDPReplies(session *forwardUDPSession, conn net.Conn) {
	defer r.wg.Done()
	defer session.stop()
	r.mu.Lock()
	idleTimeout := r.udpIdle
	r.mu.Unlock()
	buffer := make([]byte, 65535)
	for {
		_ = conn.SetReadDeadline(time.Now().Add(idleTimeout))
		n, err := conn.Read(buffer)
		if err != nil {
			if timeout, ok := err.(net.Error); ok && timeout.Timeout() &&
				time.Since(time.Unix(0, session.activity.Load())) < idleTimeout {
				continue
			}
			return
		}
		if _, err := r.udpListener.WriteTo(buffer[:n], session.client); err != nil {
			return
		}
		session.activity.Store(time.Now().UnixNano())
	}
}

func (s *forwardUDPSession) stop() {
	s.cancel()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.closed = true
	if s.conn != nil {
		_ = s.conn.Close()
	}
}
