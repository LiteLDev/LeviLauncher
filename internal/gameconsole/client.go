package gameconsole

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

func Attach(addr, token, title string, stdin io.Reader, stdout, stderr io.Writer) error {
	if !strings.HasPrefix(addr, "127.0.0.1:") {
		return fmt.Errorf("refusing non-loopback console bridge address: %s", addr)
	}
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()
	if tc, ok := conn.(*net.TCPConn); ok {
		_ = tc.SetNoDelay(true)
	}

	_ = conn.SetDeadline(time.Now().Add(handshakeTimeout))
	if _, err := fmt.Fprintf(conn, "CLIENT %s\n", token); err != nil {
		return err
	}
	reader := bufio.NewReader(conn)
	reply, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	if strings.TrimRight(reply, "\r\n") != "OK" {
		return ErrHandshakeRejected
	}
	_ = conn.SetDeadline(time.Time{})

	if title != "" {
		safeTitle := strings.NewReplacer("\x1b", "", "\x07", "", "\r", "", "\n", "").Replace(title)
		fmt.Fprintf(stdout, "\x1b]0;LeviLauncher - %s\x07", safeTitle)
		fmt.Fprintf(stdout, "\x1b[2m[LeviLauncher] Console attached: %s (Ctrl+C or close window to detach without stopping the game)\x1b[0m\n", safeTitle)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	detached := make(chan struct{})
	go func() {
		select {
		case <-sigCh:
			fmt.Fprint(stderr, "\n\x1b[2m[LeviLauncher] Console detached (game is still running).\x1b[0m\n")
			_ = conn.Close()
		case <-detached:
		}
	}()

	go func() {
		_, _ = io.Copy(conn, stdin)
		if tc, ok := conn.(*net.TCPConn); ok {
			_ = tc.CloseWrite()
		}
	}()

	_, copyErr := io.Copy(stdout, reader)
	close(detached)
	if copyErr != nil && !errorsIsClosed(copyErr) {
		return copyErr
	}
	fmt.Fprint(stdout, "\n\x1b[2m[LeviLauncher] Game console closed.\x1b[0m\n")
	return nil
}

func errorsIsClosed(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "use of closed network connection")
}

// RunCLI runs the lightweight terminal-attach entrypoint invoked by the default
// macOS terminal application before Wails or AppKit initializes.
func RunCLI(args []string) int {
	fs := flag.NewFlagSet("attach-console", flag.ContinueOnError)
	addr := fs.String("addr", "", "loopback console bridge address")
	tokenFile := fs.String("token-file", "", "path to one-time token file")
	title := fs.String("title", "", "instance title")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	token := strings.TrimSpace(os.Getenv("LEVI_CONSOLE_TOKEN"))
	if *tokenFile != "" {
		raw, err := os.ReadFile(*tokenFile)
		_ = os.Remove(*tokenFile)
		_ = os.Remove(filepath.Dir(*tokenFile))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to read console session token: %v\n", err)
			return 1
		}
		token = strings.TrimSpace(string(raw))
	}
	if *addr == "" || token == "" {
		fmt.Fprintln(os.Stderr, "Missing console bridge address or token.")
		return 2
	}
	if err := Attach(*addr, token, *title, os.Stdin, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "Console bridge error: %v\n", err)
		return 1
	}
	return 0
}
