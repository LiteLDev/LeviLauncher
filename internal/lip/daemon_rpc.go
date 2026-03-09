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
	"sync/atomic"
	"syscall"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const (
	defaultDaemonTimeout = 5 * time.Minute
	EventLipTaskStarted  = "lip_task_started"
	EventLipTaskLog      = "lip_task_log"
	EventLipTaskProgress = "lip_task_progress"
	EventLipTaskFinished = "lip_task_finished"
)

var lipTaskCounter uint64

type LipTaskStartedEvent struct {
	TaskID    string   `json:"taskId"`
	Method    string   `json:"method"`
	Target    string   `json:"target"`
	Packages  []string `json:"packages"`
	Timestamp int64    `json:"timestamp"`
}

type LipTaskLogEvent struct {
	TaskID    string   `json:"taskId"`
	Method    string   `json:"method"`
	Target    string   `json:"target"`
	Packages  []string `json:"packages"`
	Timestamp int64    `json:"timestamp"`
	Level     string   `json:"level"`
	Message   string   `json:"message"`
	Raw       string   `json:"raw"`
}

type LipTaskProgressEvent struct {
	TaskID     string   `json:"taskId"`
	Method     string   `json:"method"`
	Target     string   `json:"target"`
	Packages   []string `json:"packages"`
	Timestamp  int64    `json:"timestamp"`
	ProgressID string   `json:"progressId"`
	Message    string   `json:"message"`
	Percentage float64  `json:"percentage"`
}

type LipTaskFinishedEvent struct {
	TaskID     string   `json:"taskId"`
	Method     string   `json:"method"`
	Target     string   `json:"target"`
	Packages   []string `json:"packages"`
	Timestamp  int64    `json:"timestamp"`
	Success    bool     `json:"success"`
	Error      string   `json:"error"`
	Stderr     string   `json:"stderr"`
	DurationMs int64    `json:"durationMs"`
}

type lipTaskMeta struct {
	TaskID    string
	Method    string
	Target    string
	Packages  []string
	StartedAt time.Time
}

func nextLipTaskID() string {
	seq := atomic.AddUint64(&lipTaskCounter, 1)
	return fmt.Sprintf("lip-%d-%d", time.Now().UnixNano(), seq)
}

func emitLipTaskEvent(event string, data any) {
	app := application.Get()
	if app == nil {
		return
	}
	app.Event.Emit(event, data)
}

func currentUnixMilli() int64 {
	return time.Now().UnixMilli()
}

func normalizeTaskMethod(method string) string {
	return strings.TrimSpace(method)
}

func normalizeTaskTarget(workDir string) string {
	trimmed := strings.TrimSpace(workDir)
	if trimmed == "" {
		return ""
	}
	cleaned := filepath.Clean(trimmed)
	base := strings.TrimSpace(filepath.Base(cleaned))
	if base == "" || base == "." || base == string(filepath.Separator) {
		return cleaned
	}
	return base
}

func extractTaskPackages(params any) []string {
	toStringList := func(v any) []string {
		switch typed := v.(type) {
		case []string:
			out := make([]string, 0, len(typed))
			for _, item := range typed {
				trimmed := strings.TrimSpace(item)
				if trimmed != "" {
					out = append(out, trimmed)
				}
			}
			return out
		case []any:
			out := make([]string, 0, len(typed))
			for _, item := range typed {
				trimmed := strings.TrimSpace(fmt.Sprint(item))
				if trimmed != "" && trimmed != "<nil>" {
					out = append(out, trimmed)
				}
			}
			return out
		default:
			return nil
		}
	}

	switch typed := params.(type) {
	case []string:
		return toStringList(typed)
	case []any:
		if len(typed) == 0 {
			return nil
		}
		return toStringList(typed[0])
	default:
		return nil
	}
}

func parseProgressNumber(raw any) float64 {
	switch typed := raw.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case int32:
		return float64(typed)
	case uint:
		return float64(typed)
	case uint64:
		return float64(typed)
	case uint32:
		return float64(typed)
	case string:
		value := strings.TrimSpace(typed)
		if value == "" {
			return 0
		}
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return 0
		}
		return parsed
	default:
		return 0
	}
}

func emitTaskStarted(meta lipTaskMeta) {
	emitLipTaskEvent(EventLipTaskStarted, LipTaskStartedEvent{
		TaskID:    meta.TaskID,
		Method:    meta.Method,
		Target:    meta.Target,
		Packages:  append([]string{}, meta.Packages...),
		Timestamp: currentUnixMilli(),
	})
}

func emitTaskLog(meta lipTaskMeta, level string, message string, raw string) {
	emitLipTaskEvent(EventLipTaskLog, LipTaskLogEvent{
		TaskID:    meta.TaskID,
		Method:    meta.Method,
		Target:    meta.Target,
		Packages:  append([]string{}, meta.Packages...),
		Timestamp: currentUnixMilli(),
		Level:     strings.TrimSpace(level),
		Message:   strings.TrimSpace(message),
		Raw:       strings.TrimSpace(raw),
	})
}

func emitTaskProgress(meta lipTaskMeta, progressID string, message string, percentage float64) {
	emitLipTaskEvent(EventLipTaskProgress, LipTaskProgressEvent{
		TaskID:     meta.TaskID,
		Method:     meta.Method,
		Target:     meta.Target,
		Packages:   append([]string{}, meta.Packages...),
		Timestamp:  currentUnixMilli(),
		ProgressID: strings.TrimSpace(progressID),
		Message:    strings.TrimSpace(message),
		Percentage: percentage,
	})
}

func emitTaskFinished(meta lipTaskMeta, success bool, taskError string, stderr string) {
	duration := time.Since(meta.StartedAt).Milliseconds()
	if duration < 0 {
		duration = 0
	}
	emitLipTaskEvent(EventLipTaskFinished, LipTaskFinishedEvent{
		TaskID:     meta.TaskID,
		Method:     meta.Method,
		Target:     meta.Target,
		Packages:   append([]string{}, meta.Packages...),
		Timestamp:  currentUnixMilli(),
		Success:    success,
		Error:      strings.TrimSpace(taskError),
		Stderr:     strings.TrimSpace(stderr),
		DurationMs: duration,
	})
}

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

func logDaemonCallback(meta lipTaskMeta, method string, params json.RawMessage) {
	trimmedMethod := strings.TrimSpace(method)
	if len(params) == 0 {
		log.Printf("lipd callback: %s", trimmedMethod)
		emitTaskLog(meta, "info", fmt.Sprintf("callback: %s", trimmedMethod), "")
		return
	}
	var arr []any
	if err := json.Unmarshal(params, &arr); err == nil {
		switch trimmedMethod {
		case "PrintInfo", "PrintSuccess", "PrintWarning", "PrintError":
			if len(arr) > 0 {
				level := strings.ToLower(strings.TrimPrefix(trimmedMethod, "Print"))
				// PrintSuccess from lipd is often a step-level status, not final task completion.
				// Keep it as info to avoid misleading "already done" perception in UI.
				if trimmedMethod == "PrintSuccess" {
					level = "info"
				}
				message := fmt.Sprint(arr[0])
				log.Printf("lipd %s: %v", level, arr[0])
				emitTaskLog(meta, level, message, string(params))
				return
			}
		case "ReportProgress":
			if len(arr) >= 3 {
				progressID := fmt.Sprint(arr[0])
				message := fmt.Sprint(arr[1])
				percentage := parseProgressNumber(arr[2])
				log.Printf("lipd progress: id=%v message=%v percentage=%v", arr[0], arr[1], arr[2])
				emitTaskProgress(meta, progressID, message, percentage)
				return
			}
		}
	}
	raw := strings.TrimSpace(string(params))
	log.Printf("lipd callback: %s %s", trimmedMethod, raw)
	emitTaskLog(meta, "info", fmt.Sprintf("%s %s", trimmedMethod, raw), raw)
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
	return callDaemonWithResultInternal(ctx, workDir, method, params, true)
}

func callDaemonWithResultQuiet(ctx context.Context, workDir string, method string, params any) (json.RawMessage, error) {
	return callDaemonWithResultInternal(ctx, workDir, method, params, false)
}

func callDaemonWithResultInternal(ctx context.Context, workDir string, method string, params any, trackTask bool) (json.RawMessage, error) {
	ctx, cancel := normalizeCtx(ctx)
	defer cancel()

	taskMeta := lipTaskMeta{}
	if trackTask {
		taskMeta = lipTaskMeta{
			TaskID:    nextLipTaskID(),
			Method:    normalizeTaskMethod(method),
			Target:    normalizeTaskTarget(workDir),
			Packages:  extractTaskPackages(params),
			StartedAt: time.Now(),
		}
		emitTaskStarted(taskMeta)
	}

	if err := ensureLipRuntimeConfig(); err != nil {
		callErr := fmt.Errorf("ensure lip runtime config: %w", err)
		if trackTask {
			emitTaskFinished(taskMeta, false, callErr.Error(), "")
		}
		return nil, callErr
	}

	exePath := LipExePath()
	cmd := exec.CommandContext(ctx, exePath, "run")
	cmd.Dir = workDir
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		callErr := fmt.Errorf("open stdin pipe: %w", err)
		if trackTask {
			emitTaskFinished(taskMeta, false, callErr.Error(), "")
		}
		return nil, callErr
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		callErr := fmt.Errorf("open stdout pipe: %w", err)
		if trackTask {
			emitTaskFinished(taskMeta, false, callErr.Error(), "")
		}
		return nil, callErr
	}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		callErr := fmt.Errorf("start lipd: %w", err)
		if trackTask {
			emitTaskFinished(taskMeta, false, callErr.Error(), "")
		}
		return nil, callErr
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
		callErr := fmt.Errorf("marshal request: %w", err)
		if trackTask {
			emitTaskFinished(taskMeta, false, callErr.Error(), "")
		}
		return nil, callErr
	}

	if err := writeRPCMessage(writer, reqPayload); err != nil {
		_ = stdin.Close()
		_ = cmd.Wait()
		callErr := fmt.Errorf("write request: %w", err)
		if trackTask {
			emitTaskFinished(taskMeta, false, callErr.Error(), "")
		}
		return nil, callErr
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
			if trackTask {
				logDaemonCallback(taskMeta, msg.Method, msg.Params)
			}
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
		if trackTask {
			emitTaskFinished(taskMeta, true, "", "")
		}
		return callResult, nil
	}
	stderrText := strings.TrimSpace(stderr.String())
	if trackTask && stderrText != "" {
		for _, line := range strings.Split(stderrText, "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}
			emitTaskLog(taskMeta, "error", trimmed, trimmed)
		}
	}
	if trackTask {
		emitTaskFinished(taskMeta, false, callErr.Error(), stderrText)
	}

	if stderrText != "" {
		return nil, fmt.Errorf("%w; stderr=%s", callErr, stderrText)
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
	if err := EnsureInstalledWithError(); err != nil {
		return err
	}
	args := []any{packages, false, false, false}
	return callDaemon(ctx, filepath.Clean(workDir), "Install", args)
}

func UpdatePackagesViaDaemon(ctx context.Context, workDir string, packages []string) error {
	if strings.TrimSpace(workDir) == "" {
		return fmt.Errorf("empty work directory")
	}
	if err := EnsureInstalledWithError(); err != nil {
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

type PackageInstallState struct {
	Installed         bool
	ExplicitInstalled bool
	InstalledVersion  string
}

func parsePackageSpecFromString(raw string) (daemonPackageSpec, bool) {
	spec := strings.TrimSpace(raw)
	if spec == "" {
		return daemonPackageSpec{}, false
	}

	version := ""
	if at := strings.LastIndex(spec, "@"); at > 0 {
		version = strings.TrimSpace(spec[at+1:])
		spec = strings.TrimSpace(spec[:at])
	}
	if spec == "" {
		return daemonPackageSpec{}, false
	}

	path := spec
	variant := ""
	if hash := strings.LastIndex(spec, "#"); hash > 0 {
		path = strings.TrimSpace(spec[:hash])
		variant = strings.TrimSpace(spec[hash+1:])
	}
	if path == "" {
		return daemonPackageSpec{}, false
	}

	return daemonPackageSpec{
		ID: daemonPackageID{
			Path:    path,
			Variant: variant,
		},
		Version: version,
	}, true
}

func parsePackageSpecsFromRaw(value json.RawMessage) ([]daemonPackageSpec, bool) {
	var specs []daemonPackageSpec
	if err := json.Unmarshal(value, &specs); err == nil {
		return specs, true
	}

	var asStrings []string
	if err := json.Unmarshal(value, &asStrings); err == nil {
		converted := make([]daemonPackageSpec, 0, len(asStrings))
		for _, raw := range asStrings {
			if spec, ok := parsePackageSpecFromString(raw); ok {
				converted = append(converted, spec)
			}
		}
		return converted, true
	}

	var asMixed []any
	if err := json.Unmarshal(value, &asMixed); err == nil {
		converted := make([]daemonPackageSpec, 0, len(asMixed))
		for _, item := range asMixed {
			switch typed := item.(type) {
			case string:
				if spec, ok := parsePackageSpecFromString(typed); ok {
					converted = append(converted, spec)
				}
			case map[string]any:
				blob, err := json.Marshal(typed)
				if err != nil {
					continue
				}
				var spec daemonPackageSpec
				if err := json.Unmarshal(blob, &spec); err != nil {
					continue
				}
				if strings.TrimSpace(spec.ID.Path) == "" {
					continue
				}
				converted = append(converted, spec)
			}
		}
		return converted, true
	}

	return nil, false
}

func decodePackageSpecGroups(raw json.RawMessage) ([][]daemonPackageSpec, error) {
	var grouped [][]daemonPackageSpec
	if err := json.Unmarshal(raw, &grouped); err == nil {
		return grouped, nil
	}

	type tupleListResult struct {
		Item1 json.RawMessage `json:"Item1"`
		Item2 json.RawMessage `json:"Item2"`
	}
	var tuple tupleListResult
	if err := json.Unmarshal(raw, &tuple); err == nil && (tuple.Item1 != nil || tuple.Item2 != nil) {
		explicit, hasExplicit := parsePackageSpecsFromRaw(tuple.Item1)
		indirect, hasIndirect := parsePackageSpecsFromRaw(tuple.Item2)
		if hasExplicit || hasIndirect {
			return [][]daemonPackageSpec{explicit, indirect}, nil
		}
	}

	var keyed map[string]json.RawMessage
	if err := json.Unmarshal(raw, &keyed); err == nil {
		pickFirst := func(keys ...string) ([]daemonPackageSpec, bool) {
			for _, key := range keys {
				value, ok := keyed[key]
				if !ok {
					continue
				}
				if specs, ok := parsePackageSpecsFromRaw(value); ok {
					return specs, true
				}
			}
			return nil, false
		}

		explicit, hasExplicit := pickFirst("Item1", "Explicit", "explicit", "TopLevel", "topLevel")
		indirect, hasIndirect := pickFirst("Item2", "Dependencies", "dependencies", "Transitive", "transitive")
		if hasExplicit || hasIndirect {
			return [][]daemonPackageSpec{explicit, indirect}, nil
		}
	}

	var oneDimension []daemonPackageSpec
	if specs, ok := parsePackageSpecsFromRaw(raw); ok {
		oneDimension = specs
		return [][]daemonPackageSpec{oneDimension}, nil
	}

	preview := strings.TrimSpace(string(raw))
	if len(preview) > 256 {
		preview = preview[:256] + "..."
	}
	return nil, fmt.Errorf("unsupported list result format: %s", preview)
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

func GetPackageInstallStateViaDaemon(ctx context.Context, workDir string, packageRef string) (PackageInstallState, error) {
	state := PackageInstallState{}
	if strings.TrimSpace(workDir) == "" {
		return state, fmt.Errorf("empty work directory")
	}
	if err := EnsureInstalledWithError(); err != nil {
		return state, err
	}

	result, err := callDaemonWithResult(ctx, filepath.Clean(workDir), "List", []any{})
	if err != nil {
		return state, err
	}

	grouped, err := decodePackageSpecGroups(result)
	if err != nil {
		return state, fmt.Errorf("decode list result: %w", err)
	}

	target := strings.ToLower(trimPackageVersion(packageRef))
	if target == "" {
		return state, nil
	}

	parseVersion := func(spec daemonPackageSpec) string {
		return strings.TrimSpace(spec.Version)
	}

	checkMatches := func(specs []daemonPackageSpec, explicit bool) {
		for _, spec := range specs {
			if strings.ToLower(packageSpecID(spec)) != target {
				continue
			}
			state.Installed = true
			if explicit {
				state.ExplicitInstalled = true
				state.InstalledVersion = parseVersion(spec)
				continue
			}
			if state.InstalledVersion == "" {
				state.InstalledVersion = parseVersion(spec)
			}
		}
	}

	if len(grouped) > 0 {
		checkMatches(grouped[0], true)
	}
	for i := 1; i < len(grouped); i++ {
		checkMatches(grouped[i], false)
	}

	return state, nil
}

func IsPackageExplicitlyInstalledViaDaemon(ctx context.Context, workDir string, packageRef string) (bool, error) {
	state, err := GetPackageInstallStateViaDaemon(ctx, workDir, packageRef)
	if err != nil {
		return false, err
	}
	return state.ExplicitInstalled, nil
}

func UninstallPackagesViaDaemon(ctx context.Context, workDir string, packages []string, noDependencies bool) error {
	if strings.TrimSpace(workDir) == "" {
		return fmt.Errorf("empty work directory")
	}
	if err := EnsureInstalledWithError(); err != nil {
		return err
	}
	args := []any{packages, noDependencies, false, false}
	return callDaemon(ctx, filepath.Clean(workDir), "Uninstall", args)
}
