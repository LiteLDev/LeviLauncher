package gameconsole

import (
	"bufio"
	"bytes"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

func dialHandshake(t *testing.T, addr, role, token string) (net.Conn, *bufio.Reader) {
	t.Helper()
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		t.Fatalf("dial %s: %v", addr, err)
	}
	if _, err := conn.Write([]byte(role + " " + token + "\n")); err != nil {
		conn.Close()
		t.Fatalf("write handshake: %v", err)
	}
	r := bufio.NewReader(conn)
	line, err := r.ReadString('\n')
	if err != nil || line != "OK\n" {
		conn.Close()
		t.Fatalf("expected OK, got %q (err=%v)", line, err)
	}
	return conn, r
}

func TestBrokerBidirectionalAndRingReplay(t *testing.T) {
	b, err := Start()
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer b.Close()

	s := b.Register("Release 1.26")
	if s == nil {
		t.Fatal("Register returned nil")
	}
	defer s.Close()

	// Reject wrong token.
	badConn, err := net.DialTimeout("tcp", s.Address(), 2*time.Second)
	if err != nil {
		t.Fatalf("dial bad: %v", err)
	}
	_, _ = badConn.Write([]byte("GAME wrong:token\n"))
	_ = badConn.SetReadDeadline(time.Now().Add(time.Second))
	buf := make([]byte, 16)
	if n, err := badConn.Read(buf); err == nil && n > 0 {
		t.Fatalf("expected bad handshake to be rejected, got %q", string(buf[:n]))
	}
	_ = badConn.Close()

	// Game connects and emits early boot logs BEFORE any terminal attaches.
	gameConn, gameReader := dialHandshake(t, s.Address(), "GAME", s.AuthToken())
	defer gameConn.Close()
	if _, err := gameConn.Write([]byte("boot-line-1\n\x1b[32mboot-line-2\x1b[0m\n")); err != nil {
		t.Fatalf("game write: %v", err)
	}
	time.Sleep(50 * time.Millisecond)

	// Terminal attaches late: should receive ring buffer replay, then live bidirectional I/O.
	clientConn, clientReader := dialHandshake(t, s.Address(), "CLIENT", s.AuthToken())
	_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	line1, err := clientReader.ReadString('\n')
	if err != nil || line1 != "boot-line-1\n" {
		t.Fatalf("line1 = %q, err = %v", line1, err)
	}
	line2, err := clientReader.ReadString('\n')
	if err != nil || line2 != "\x1b[32mboot-line-2\x1b[0m\n" {
		t.Fatalf("line2 = %q, err = %v", line2, err)
	}

	// Send interactive command from client -> game.
	if _, err := clientConn.Write([]byte("help\n")); err != nil {
		t.Fatalf("client write: %v", err)
	}
	_ = gameConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	cmdLine, err := gameReader.ReadString('\n')
	if err != nil || cmdLine != "help\n" {
		t.Fatalf("game got %q, err = %v", cmdLine, err)
	}

	// Client detaches: game connection must remain alive and accept a re-attach.
	_ = clientConn.Close()
	time.Sleep(50 * time.Millisecond)
	if _, err := gameConn.Write([]byte("after-detach\n")); err != nil {
		t.Fatalf("game write after detach: %v", err)
	}
	time.Sleep(50 * time.Millisecond)

	// Second client attaches and verifies Attach() exits cleanly when game closes.
	inR, inW := io.Pipe()
	defer inW.Close()
	var outBuf, errBuf bytes.Buffer
	attachDone := make(chan error, 1)
	go func() {
		attachDone <- Attach(s.Address(), s.AuthToken(), "TestInst", inR, &outBuf, &errBuf)
	}()

	time.Sleep(80 * time.Millisecond)
	_, _ = inW.Write([]byte("stop\n"))
	_ = gameConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	stopLine, err := gameReader.ReadString('\n')
	if err != nil || stopLine != "stop\n" {
		t.Fatalf("game expected stop, got %q (err=%v)", stopLine, err)
	}
	_, _ = gameConn.Write([]byte("bye\n"))
	_ = gameConn.Close()

	select {
	case err := <-attachDone:
		if err != nil {
			t.Fatalf("Attach returned error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Attach did not exit after game closed")
	}
	got := outBuf.String()
	for _, want := range []string{"boot-line-1\n", "after-detach\n", "bye\n", "Game console closed."} {
		if !strings.Contains(got, want) {
			t.Fatalf("Attach output missing %q; full output:\n%s", want, got)
		}
	}
}
