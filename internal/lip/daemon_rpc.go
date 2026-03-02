package lip

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	defaultDaemonTimeout = 5 * time.Minute
)

type rpcEnvelope struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int64  `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result"`
	Error   *rpcError       `json:"error,omitempty"`
}

func writeRPCMessage(w *bufio.Writer, payload []byte) error {
	if _, err := fmt.Fprintf(w, "Content-Length: %d\r\n\r\n", len(payload)); err != nil {
		return err
	}
	if _, err := w.Write(payload); err != nil {
		return err
	}
	return w.Flush()
}

func readRPCMessage(r *bufio.Reader) ([]byte, error) {
	contentLength := -1
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(parts[0]))
		value := strings.TrimSpace(parts[1])
		if key != "content-length" {
			continue
		}
		n, err := strconv.Atoi(value)
		if err != nil {
			return nil, fmt.Errorf("invalid content-length %q: %w", value, err)
		}
		contentLength = n
	}
	if contentLength < 0 {
		return nil, fmt.Errorf("no content-length in rpc header")
	}
	body := make([]byte, contentLength)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, err
	}
	return body, nil
}

func normalizeCtx(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		return context.WithTimeout(context.Background(), defaultDaemonTimeout)
	}
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, defaultDaemonTimeout)
}

func logDaemonCallback(method string, params json.RawMessage) {
	if len(params) == 0 {
		log.Printf("lipd callback: %s", method)
		return
	}
	var arr []any
	if err := json.Unmarshal(params, &arr); err == nil {
		switch method {
		case "PrintInfo", "PrintSuccess", "PrintWarning", "PrintError":
			if len(arr) > 0 {
				log.Printf("lipd %s: %v", strings.ToLower(strings.TrimPrefix(method, "Print")), arr[0])
				return
			}
		case "ReportProgress":
			if len(arr) >= 3 {
				log.Printf("lipd progress: id=%v message=%v percentage=%v", arr[0], arr[1], arr[2])
				return
			}
		}
	}
	log.Printf("lipd callback: %s %s", method, string(params))
}

func isExpectedID(raw json.RawMessage, want int64) bool {
	if len(raw) == 0 {
		return false
	}
	var asInt int64
	if err := json.Unmarshal(raw, &asInt); err == nil {
		return asInt == want
	}
	var asFloat float64
	if err := json.Unmarshal(raw, &asFloat); err == nil {
		return int64(asFloat) == want
	}
	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		if parsed, err := strconv.ParseInt(strings.TrimSpace(asString), 10, 64); err == nil {
			return parsed == want
		}
	}
	return false
}

func callDaemonWithResult(ctx context.Context, workDir string, method string, params any) (json.RawMessage, error) {
	ctx, cancel := normalizeCtx(ctx)
	defer cancel()

	exePath := LipExePath()
	cmd := exec.CommandContext(ctx, exePath, "run")
	cmd.Dir = workDir
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("open stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, fmt.Errorf("open stdout pipe: %w", err)
	}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		return nil, fmt.Errorf("start lipd: %w", err)
	}

	reader := bufio.NewReader(stdout)
	writer := bufio.NewWriter(stdin)

	reqID := int64(1)
	reqPayload, err := json.Marshal(rpcRequest{
		JSONRPC: "2.0",
		ID:      reqID,
		Method:  method,
		Params:  params,
	})
	if err != nil {
		_ = stdin.Close()
		_ = cmd.Wait()
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	if err := writeRPCMessage(writer, reqPayload); err != nil {
		_ = stdin.Close()
		_ = cmd.Wait()
		return nil, fmt.Errorf("write request: %w", err)
	}

	var callErr error
	var callResult json.RawMessage
	for {
		type readResult struct {
			payload []byte
			err     error
		}
		readCh := make(chan readResult, 1)
		go func() {
			msgPayload, err := readRPCMessage(reader)
			readCh <- readResult{payload: msgPayload, err: err}
		}()

		var msgPayload []byte
		select {
		case <-ctx.Done():
			callErr = ctx.Err()
		case read := <-readCh:
			if read.err != nil {
				callErr = fmt.Errorf("read rpc message: %w", read.err)
				break
			}
			msgPayload = read.payload
		}
		if callErr != nil {
			break
		}

		var msg rpcEnvelope
		if err := json.Unmarshal(msgPayload, &msg); err != nil {
			callErr = fmt.Errorf("unmarshal rpc message: %w", err)
			break
		}

		if strings.TrimSpace(msg.Method) != "" {
			logDaemonCallback(msg.Method, msg.Params)
			if len(msg.ID) != 0 {
				respPayload, err := json.Marshal(rpcResponse{
					JSONRPC: "2.0",
					ID:      msg.ID,
					Result:  nil,
				})
				if err != nil {
					callErr = fmt.Errorf("marshal callback response: %w", err)
					break
				}
				if err := writeRPCMessage(writer, respPayload); err != nil {
					callErr = fmt.Errorf("write callback response: %w", err)
					break
				}
			}
			continue
		}

		if !isExpectedID(msg.ID, reqID) {
			continue
		}

		if msg.Error != nil {
			callErr = fmt.Errorf("rpc %s failed: code=%d message=%s", method, msg.Error.Code, msg.Error.Message)
		} else {
			callResult = msg.Result
		}
		break
	}

	_ = stdin.Close()

	waitDone := make(chan error, 1)
	go func() {
		waitDone <- cmd.Wait()
	}()

	select {
	case waitErr := <-waitDone:
		if callErr == nil && waitErr != nil && ctx.Err() == nil {
			callErr = fmt.Errorf("lipd exited: %w", waitErr)
		}
	case <-time.After(2 * time.Second):
		_ = cmd.Process.Kill()
		<-waitDone
	}

	if callErr == nil {
		return callResult, nil
	}
	if s := strings.TrimSpace(stderr.String()); s != "" {
		return nil, fmt.Errorf("%w; stderr=%s", callErr, s)
	}
	return nil, callErr
}

func callDaemon(ctx context.Context, workDir string, method string, params any) error {
	_, err := callDaemonWithResult(ctx, workDir, method, params)
	return err
}

func InstallPackagesViaDaemon(ctx context.Context, workDir string, packages []string) error {
	if strings.TrimSpace(workDir) == "" {
		return fmt.Errorf("empty work directory")
	}
	if err := EnsureLatestWithError(ctx); err != nil {
		return err
	}
	args := []any{packages, false, false, false}
	return callDaemon(ctx, filepath.Clean(workDir), "Install", args)
}

func UpdatePackagesViaDaemon(ctx context.Context, workDir string, packages []string) error {
	if strings.TrimSpace(workDir) == "" {
		return fmt.Errorf("empty work directory")
	}
	if err := EnsureLatestWithError(ctx); err != nil {
		return err
	}
	args := []any{packages, false, false}
	return callDaemon(ctx, filepath.Clean(workDir), "Update", args)
}

type daemonPackageID struct {
	Path    string `json:"Path"`
	Variant string `json:"Variant"`
}

type daemonPackageSpec struct {
	ID      daemonPackageID `json:"Id"`
	Version string          `json:"Version"`
}

func trimPackageVersion(spec string) string {
	trimmed := strings.TrimSpace(spec)
	if trimmed == "" {
		return ""
	}
	idx := strings.LastIndex(trimmed, "@")
	if idx <= 0 {
		return trimmed
	}
	return strings.TrimSpace(trimmed[:idx])
}

func packageSpecID(spec daemonPackageSpec) string {
	path := strings.TrimSpace(spec.ID.Path)
	variant := strings.TrimSpace(spec.ID.Variant)
	if path == "" {
		return ""
	}
	if variant == "" {
		return path
	}
	return path + "#" + variant
}

func IsPackageExplicitlyInstalledViaDaemon(ctx context.Context, workDir string, packageRef string) (bool, error) {
	if strings.TrimSpace(workDir) == "" {
		return false, fmt.Errorf("empty work directory")
	}
	if err := EnsureLatestWithError(ctx); err != nil {
		return false, err
	}

	result, err := callDaemonWithResult(ctx, filepath.Clean(workDir), "List", []any{})
	if err != nil {
		return false, err
	}

	var grouped [][]daemonPackageSpec
	if err := json.Unmarshal(result, &grouped); err != nil {
		return false, fmt.Errorf("decode list result: %w", err)
	}
	if len(grouped) == 0 {
		return false, nil
	}

	target := strings.ToLower(trimPackageVersion(packageRef))
	if target == "" {
		return false, nil
	}
	for _, spec := range grouped[0] { // first list is explicit-installed packages
		if strings.ToLower(packageSpecID(spec)) == target {
			return true, nil
		}
	}
	return false, nil
}

func UninstallPackagesViaDaemon(ctx context.Context, workDir string, packages []string) error {
	if strings.TrimSpace(workDir) == "" {
		return fmt.Errorf("empty work directory")
	}
	if err := EnsureLatestWithError(ctx); err != nil {
		return err
	}
	args := []any{packages, false, false, false}
	return callDaemon(ctx, filepath.Clean(workDir), "Uninstall", args)
}
