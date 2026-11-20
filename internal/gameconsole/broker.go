// Package gameconsole bridges Windows console I/O from Wine into the host's
// default macOS terminal application over an authenticated loopback TCP socket.
package gameconsole

import (
	"bufio"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"time"
)

const (
	ringBufferSize   = 64 * 1024
	pendingInputSize = 4 * 1024
	handshakeTimeout = 5 * time.Second
)

type Broker struct {
	mu       sync.Mutex
	listener net.Listener
	address  string
	sessions map[string]*Session
	closed   bool
}

type Session struct {
	broker    *Broker
	id        string
	token     string
	instance  string
	mu        sync.Mutex
	ring      []byte
	pendingIn []byte
	gameConn  net.Conn
	gameWrite sync.Mutex
	hadGame   bool
	clients   map[net.Conn]struct{}
	closed    bool
}

var (
	defaultMu     sync.RWMutex
	defaultBroker *Broker
)

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func Start() (*Broker, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	b := &Broker{
		listener: ln,
		address:  ln.Addr().String(),
		sessions: make(map[string]*Session),
	}
	defaultMu.Lock()
	defaultBroker = b
	defaultMu.Unlock()
	go b.serve()
	return b, nil
}

func (b *Broker) Address() string {
	return b.address
}

func (b *Broker) Close() {
	defaultMu.Lock()
	if defaultBroker == b {
		defaultBroker = nil
	}
	defaultMu.Unlock()

	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return
	}
	b.closed = true
	sessions := make([]*Session, 0, len(b.sessions))
	for _, s := range b.sessions {
		sessions = append(sessions, s)
	}
	b.sessions = make(map[string]*Session)
	b.mu.Unlock()

	_ = b.listener.Close()
	for _, s := range sessions {
		s.Close()
	}
}

func Register(instance string) *Session {
	defaultMu.RLock()
	b := defaultBroker
	defaultMu.RUnlock()
	if b == nil {
		return nil
	}
	return b.Register(instance)
}

func (b *Broker) Register(instance string) *Session {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil
	}
	s := &Session{
		broker:   b,
		id:       randomHex(16),
		token:    randomHex(24),
		instance: instance,
		clients:  make(map[net.Conn]struct{}),
	}
	b.sessions[s.id] = s
	return s
}

func (s *Session) Address() string {
	if s == nil || s.broker == nil {
		return ""
	}
	return s.broker.address
}

func (s *Session) AuthToken() string {
	if s == nil {
		return ""
	}
	return s.id + ":" + s.token
}

func (s *Session) Instance() string {
	if s == nil {
		return ""
	}
	return s.instance
}

func (s *Session) Environment() map[string]string {
	if s == nil {
		return nil
	}
	return map[string]string{
		"LEVI_CONSOLE_ADDR":  s.Address(),
		"LEVI_CONSOLE_TOKEN": s.AuthToken(),
	}
}

func (s *Session) Close() {
	if s == nil {
		return
	}
	if s.broker != nil {
		s.broker.mu.Lock()
		delete(s.broker.sessions, s.id)
		s.broker.mu.Unlock()
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	gameConn := s.gameConn
	s.gameConn = nil
	clients := make([]net.Conn, 0, len(s.clients))
	for c := range s.clients {
		clients = append(clients, c)
	}
	s.clients = make(map[net.Conn]struct{})
	s.mu.Unlock()

	if gameConn != nil {
		_ = gameConn.Close()
	}
	for _, c := range clients {
		_ = c.Close()
	}
}

func (b *Broker) serve() {
	for {
		conn, err := b.listener.Accept()
		if err != nil {
			return
		}
		go b.handleConn(conn)
	}
}

func (b *Broker) authenticate(auth string) *Session {
	id, tok, ok := strings.Cut(auth, ":")
	if !ok || id == "" || tok == "" {
		return nil
	}
	b.mu.Lock()
	s := b.sessions[id]
	b.mu.Unlock()
	if s == nil {
		return nil
	}
	if subtle.ConstantTimeCompare([]byte(tok), []byte(s.token)) != 1 {
		return nil
	}
	return s
}

func (b *Broker) handleConn(conn net.Conn) {
	if tc, ok := conn.(*net.TCPConn); ok {
		_ = tc.SetNoDelay(true)
	}
	_ = conn.SetReadDeadline(time.Now().Add(handshakeTimeout))
	reader := bufio.NewReader(io.LimitReader(conn, 512))
	line, err := reader.ReadString('\n')
	if err != nil {
		_ = conn.Close()
		return
	}
	_ = conn.SetReadDeadline(time.Time{})

	role, auth, ok := strings.Cut(strings.TrimRight(line, "\r\n"), " ")
	if !ok {
		_ = conn.Close()
		return
	}
	session := b.authenticate(auth)
	if session == nil {
		_ = conn.Close()
		return
	}

	// Drain any bytes buffered by bufio.Reader beyond the handshake line.
	var leftover []byte
	if buffered := reader.Buffered(); buffered > 0 {
		leftover = make([]byte, buffered)
		if _, err := io.ReadFull(reader, leftover); err != nil {
			_ = conn.Close()
			return
		}
	}

	switch role {
	case "GAME":
		session.handleGame(conn, leftover)
	case "CLIENT":
		session.handleClient(conn, leftover)
	default:
		_ = conn.Close()
	}
}

func (s *Session) handleGame(conn net.Conn, leftover []byte) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		_ = conn.Close()
		return
	}
	prev := s.gameConn
	s.gameConn = conn
	s.hadGame = true
	pending := s.pendingIn
	s.pendingIn = nil
	s.mu.Unlock()

	if prev != nil {
		_ = prev.Close()
	}
	if _, err := conn.Write([]byte("OK\n")); err != nil {
		_ = conn.Close()
		return
	}
	if len(pending) > 0 {
		s.writeToGame(conn, pending)
	}
	if len(leftover) > 0 {
		s.broadcast(leftover)
	}

	buf := make([]byte, 4096)
	for {
		n, err := conn.Read(buf)
		if n > 0 {
			s.broadcast(buf[:n])
		}
		if err != nil {
			break
		}
	}

	s.mu.Lock()
	shouldCloseClients := s.gameConn == conn
	if shouldCloseClients {
		s.gameConn = nil
	}
	var clients []net.Conn
	if shouldCloseClients && s.hadGame {
		clients = make([]net.Conn, 0, len(s.clients))
		for c := range s.clients {
			clients = append(clients, c)
		}
		s.clients = make(map[net.Conn]struct{})
	}
	s.mu.Unlock()

	_ = conn.Close()
	for _, c := range clients {
		_ = c.Close()
	}
}

func (s *Session) handleClient(conn net.Conn, leftover []byte) {
	s.mu.Lock()
	if s.closed || (s.hadGame && s.gameConn == nil) {
		s.mu.Unlock()
		_ = conn.Close()
		return
	}
	snapshot := append([]byte(nil), s.ring...)
	s.clients[conn] = struct{}{}
	s.mu.Unlock()

	if _, err := conn.Write([]byte("OK\n")); err != nil {
		s.removeClient(conn)
		return
	}
	if len(snapshot) > 0 {
		if _, err := conn.Write(snapshot); err != nil {
			s.removeClient(conn)
			return
		}
	}
	if len(leftover) > 0 {
		s.forwardInput(leftover)
	}

	buf := make([]byte, 4096)
	for {
		n, err := conn.Read(buf)
		if n > 0 {
			s.forwardInput(buf[:n])
		}
		if err != nil {
			break
		}
	}
	s.removeClient(conn)
}

func (s *Session) removeClient(conn net.Conn) {
	s.mu.Lock()
	delete(s.clients, conn)
	s.mu.Unlock()
	_ = conn.Close()
}

func (s *Session) broadcast(data []byte) {
	s.mu.Lock()
	if len(data) >= ringBufferSize {
		s.ring = append(s.ring[:0], data[len(data)-ringBufferSize:]...)
	} else {
		if overflow := len(s.ring) + len(data) - ringBufferSize; overflow > 0 {
			s.ring = append(s.ring[:0], s.ring[overflow:]...)
		}
		s.ring = append(s.ring, data...)
	}
	clients := make([]net.Conn, 0, len(s.clients))
	for c := range s.clients {
		clients = append(clients, c)
	}
	s.mu.Unlock()

	for _, c := range clients {
		if _, err := c.Write(data); err != nil {
			s.removeClient(c)
		}
	}
}

func (s *Session) forwardInput(data []byte) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	conn := s.gameConn
	if conn == nil {
		if len(s.pendingIn)+len(data) <= pendingInputSize {
			s.pendingIn = append(s.pendingIn, data...)
		}
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()
	s.writeToGame(conn, data)
}

func (s *Session) writeToGame(conn net.Conn, data []byte) {
	s.gameWrite.Lock()
	defer s.gameWrite.Unlock()
	_, _ = conn.Write(data)
}

var ErrHandshakeRejected = errors.New("console bridge rejected handshake")
