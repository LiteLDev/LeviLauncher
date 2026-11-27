package gameshell

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func postJSON(b *Broker, path, token, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodPost, "http://"+b.address+path, strings.NewReader(body))
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	b.ServeHTTP(w, r)
	return w
}

func TestOpenDirectoryAndRevealValidation(t *testing.T) {
	prefix := t.TempDir()
	shaderDir := filepath.Join(prefix, "drive_c", "users", "steam", "shaderpacks")
	if err := os.MkdirAll(shaderDir, 0o755); err != nil {
		t.Fatal(err)
	}
	sampleFile := filepath.Join(shaderDir, "BSL 中文.zip")
	if err := os.WriteFile(sampleFile, []byte("zip"), 0o644); err != nil {
		t.Fatal(err)
	}

	var openedDirs []string
	var revealed []string
	b, err := Start(Handlers{
		OpenDir:    func(dir string) error { openedDirs = append(openedDirs, dir); return nil },
		RevealPath: func(p string) error { revealed = append(revealed, p); return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(b.Close)

	s := Register("123中文", prefix)
	t.Cleanup(s.Close)

	endpoint := "/v1/shell/" + s.id + "/open"

	// Unauthorized & invalid payloads rejected.
	if w := postJSON(b, endpoint, "wrong", `{"path":"`+shaderDir+`"}`); w.Code != http.StatusUnauthorized {
		t.Fatalf("wrong token status=%d", w.Code)
	}
	if w := postJSON(b, endpoint, s.token, `{"path":"relative/dir"}`); w.Code != http.StatusBadRequest {
		t.Fatalf("relative path status=%d", w.Code)
	}
	if w := postJSON(b, endpoint, s.token, `{"path":"`+sampleFile+`","reveal":false}`); w.Code != http.StatusBadRequest {
		t.Fatalf("file without reveal status=%d", w.Code)
	}
	if w := postJSON(b, endpoint, s.token, `{"path":"`+filepath.Join(shaderDir, "missing")+`"}`); w.Code != http.StatusBadRequest {
		t.Fatalf("missing dir status=%d", w.Code)
	}

	// Valid POSIX directory open.
	body, _ := json.Marshal(map[string]any{"path": shaderDir, "reveal": false})
	if w := postJSON(b, endpoint, s.token, string(body)); w.Code != http.StatusOK {
		t.Fatalf("open dir status=%d body=%s", w.Code, w.Body.String())
	}

	// Valid Windows path fallback via drive_c.
	body, _ = json.Marshal(map[string]any{"windowsPath": `C:\users\steam\shaderpacks`, "reveal": false})
	if w := postJSON(b, endpoint, s.token, string(body)); w.Code != http.StatusOK {
		t.Fatalf("open windows dir status=%d body=%s", w.Code, w.Body.String())
	}

	// Valid file reveal.
	body, _ = json.Marshal(map[string]any{"path": sampleFile, "reveal": true})
	if w := postJSON(b, endpoint, s.token, string(body)); w.Code != http.StatusOK {
		t.Fatalf("reveal file status=%d body=%s", w.Code, w.Body.String())
	}

	if !reflect.DeepEqual(openedDirs, []string{shaderDir, shaderDir}) {
		t.Fatalf("openedDirs=%v", openedDirs)
	}
	if !reflect.DeepEqual(revealed, []string{sampleFile}) {
		t.Fatalf("revealed=%v", revealed)
	}

	s.Close()
	if w := postJSON(b, endpoint, s.token, string(body)); w.Code != http.StatusUnauthorized {
		t.Fatalf("closed session status=%d", w.Code)
	}
}

func TestDialogBridgeHandling(t *testing.T) {
	prefix := t.TempDir()
	initDir := filepath.Join(prefix, "drive_c", "configs")
	if err := os.MkdirAll(initDir, 0o755); err != nil {
		t.Fatal(err)
	}

	var gotReq DialogRequest
	b, err := Start(Handlers{
		ShowDialog: func(req DialogRequest) (DialogResult, error) {
			gotReq = req
			return DialogResult{
				Cancelled: false,
				Paths:     []string{filepath.Join(initDir, "shader.txt")},
			}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(b.Close)

	s := Register("inst", prefix)
	t.Cleanup(s.Close)

	endpoint := "/v1/shell/" + s.id + "/dialog"
	if w := postJSON(b, endpoint, s.token, `{"kind":"bogus"}`); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid kind status=%d", w.Code)
	}

	payload, _ := json.Marshal(DialogRequest{
		Kind:        "open",
		Title:       "选择配置",
		WindowsDir:  `C:\configs`,
		DefaultName: "shader.txt",
		DefaultExt:  ".txt",
		Multi:       true,
		Filters: []Filter{
			{Name: "Text", Patterns: []string{"*.txt;*.json"}},
			{Name: "All", Patterns: []string{"*.*"}},
		},
	})
	w := postJSON(b, endpoint, s.token, string(payload))
	if w.Code != http.StatusOK {
		t.Fatalf("dialog status=%d body=%s", w.Code, w.Body.String())
	}
	if gotReq.InitialDir != initDir || gotReq.DefaultExt != "txt" || !gotReq.Multi {
		t.Fatalf("unexpected dialog req: %+v", gotReq)
	}
	var res DialogResult
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if res.Cancelled || len(res.Paths) != 1 || res.Paths[0] != filepath.Join(initDir, "shader.txt") {
		t.Fatalf("unexpected dialog res: %+v", res)
	}
}

func TestNormalizeDialogFilters(t *testing.T) {
	exts, allowOther := NormalizeDialogFilters([]Filter{
		{Name: "Shaders", Patterns: []string{"*.zip; *.TXT", ".json"}},
		{Name: "All", Patterns: []string{"*.*"}},
	})
	if !allowOther {
		t.Fatal("expected allowOther=true for *.*")
	}
	want := []string{"zip", "TXT", "json"}
	if !reflect.DeepEqual(exts, want) {
		t.Fatalf("exts=%v want=%v", exts, want)
	}

	exts, allowOther = NormalizeDialogFilters([]Filter{
		{Name: "Configs", Patterns: []string{"*.properties"}},
	})
	if allowOther || !reflect.DeepEqual(exts, []string{"properties"}) {
		t.Fatalf("exts=%v allowOther=%v", exts, allowOther)
	}
}

func TestConcurrentDialogFocusesActiveAndCancelsDuplicate(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	focused := make(chan struct{}, 1)
	showCount := 0

	b, err := Start(Handlers{
		ShowDialog: func(req DialogRequest) (DialogResult, error) {
			showCount++
			close(started)
			<-release
			return DialogResult{Cancelled: false, Paths: []string{"/tmp/ok.txt"}}, nil
		},
		FocusDialog: func() {
			focused <- struct{}{}
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(b.Close)

	s := Register("inst", t.TempDir())
	t.Cleanup(s.Close)
	endpoint := "/v1/shell/" + s.id + "/dialog"

	firstDone := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		firstDone <- postJSON(b, endpoint, s.token, `{"kind":"open"}`)
	}()

	<-started

	// Second click while panel is open should immediately return cancelled and focus active panel.
	w2 := postJSON(b, endpoint, s.token, `{"kind":"open"}`)
	if w2.Code != http.StatusOK {
		t.Fatalf("duplicate status=%d body=%s", w2.Code, w2.Body.String())
	}
	var res2 DialogResult
	if err := json.Unmarshal(w2.Body.Bytes(), &res2); err != nil {
		t.Fatal(err)
	}
	if !res2.Cancelled || len(res2.Paths) != 0 {
		t.Fatalf("duplicate res=%+v want cancelled", res2)
	}
	select {
	case <-focused:
	default:
		t.Fatal("expected FocusDialog to be called for duplicate request")
	}

	close(release)
	w1 := <-firstDone
	if w1.Code != http.StatusOK {
		t.Fatalf("first status=%d", w1.Code)
	}
	if showCount != 1 {
		t.Fatalf("showCount=%d want 1", showCount)
	}
}
