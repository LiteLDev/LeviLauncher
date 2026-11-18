package gameauth

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testBroker(t *testing.T) (*Broker, *Session, chan State, chan string) {
	t.Helper()
	events := make(chan State, 16)
	opened := make(chan string, 8)
	b, err := Start(func(s State) { events <- s }, func(u string) error { opened <- u; return nil })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(b.Close)
	s := Register("instance", t.TempDir())
	if err := os.MkdirAll(filepath.Dir(s.statusPath), 0700); err != nil {
		t.Fatal(err)
	}
	return b, s, events, opened
}

func postChallenge(b *Broker, s *Session, body, token string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodPost, "http://"+b.address+"/v1/game-login/"+s.id, strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer "+token)
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	b.ServeHTTP(w, r)
	return w
}

func TestGameChallengeAuthenticationValidationAndDeduplication(t *testing.T) {
	b, s, events, opened := testBroker(t)
	valid := `{"code":"AB12CD34","expiresIn":900}`
	for _, tc := range []struct {
		body, token string
		status      int
	}{
		{valid, "wrong", 401},
		{`{"code":"bad","expiresIn":900}`, s.token, 400},
		{`{"code":"AB12CD34","expiresIn":901}`, s.token, 400},
		{`{"code":"AB12CD34","expiresIn":900,"verificationURL":"https://example.com"}`, s.token, 400},
		{valid + valid, s.token, 400},
	} {
		if w := postChallenge(b, s, tc.body, tc.token); w.Code != tc.status {
			t.Fatalf("status=%d want=%d", w.Code, tc.status)
		}
	}
	select {
	case <-opened:
		t.Fatal("rejected challenge opened browser")
	default:
	}
	if w := postChallenge(b, s, valid, s.token); w.Code != 200 {
		t.Fatal(w.Code)
	}
	state := <-events
	if state.Phase != "pending" || state.UserCode != "AB12CD34" || state.Instance != "instance" {
		t.Fatalf("state=%+v", state)
	}
	select {
	case u := <-opened:
		if u != "https://www.microsoft.com/link?otc=AB12CD34" {
			t.Fatal(u)
		}
	case <-time.After(time.Second):
		t.Fatal("browser did not open")
	}
	if w := postChallenge(b, s, valid, s.token); w.Code != 200 {
		t.Fatal(w.Code)
	}
	select {
	case <-opened:
		t.Fatal("duplicate challenge reopened browser")
	case <-time.After(50 * time.Millisecond):
	}
	s.Close()
	if w := postChallenge(b, s, valid, s.token); w.Code != 401 {
		t.Fatal("ended session accepted challenge")
	}
}

func TestGameResultDoesNotExposeIdentityOrCredentials(t *testing.T) {
	b, s, events, _ := testBroker(t)
	if err := os.WriteFile(s.statusPath, []byte("old status"), 0600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Minute)
	_ = os.Chtimes(s.statusPath, old, old)
	if w := postChallenge(b, s, `{"code":"AB12CD34","expiresIn":900}`, s.token); w.Code != 200 {
		t.Fatal(w.Code)
	}
	pending := <-events
	if err := os.WriteFile(s.statusPath, []byte("Signed in as private-gamertag (xuid 123456789)\r\n"), 0600); err != nil {
		t.Fatal(err)
	}
	select {
	case state := <-events:
		data, _ := json.Marshal(state)
		if state.Phase != "success" || state.UserCode != "" || state.Revision <= pending.Revision || bytes.Contains(data, []byte("private-gamertag")) || bytes.Contains(data, []byte("123456789")) {
			t.Fatalf("unsafe status: %s", data)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("game result not observed")
	}
}

func TestGameSessionCloseAndFailureStates(t *testing.T) {
	b, s, events, _ := testBroker(t)
	_ = postChallenge(b, s, `{"code":"AB12CD34","expiresIn":900}`, s.token)
	<-events
	s.Close()
	state := <-events
	if state.Phase != "closed" || state.UserCode != "" {
		t.Fatalf("closed state=%+v", state)
	}
	if phase, code := terminalStatus("Sign-in did not complete (hr=80004005)\r\n"); phase != "failed" || code != "80004005" {
		t.Fatal(phase, code)
	}
	if phase, _ := terminalStatus("Microsoft account sign-in for Minecraft"); phase != "" {
		t.Fatal("pending prompt treated as completed")
	}
}

func TestResultEventsAreBoundToTheirGameSession(t *testing.T) {
	b, s, events, _ := testBroker(t)
	other := Register("other game", t.TempDir())
	result := func(session *Session, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, "http://"+b.address+"/v1/game-login/"+session.id+"/result", strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer "+session.token)
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		b.ServeHTTP(w, r)
		return w
	}
	if w := result(s, `{"phase":"success"}`); w.Code != 409 {
		t.Fatal("accepted result before a challenge")
	}
	if w := postChallenge(b, s, `{"code":"AB12CD34","expiresIn":900,"resultEvents":true}`, s.token); w.Code != 200 {
		t.Fatal(w.Code)
	}
	pending := <-events
	if w := result(other, `{"phase":"success"}`); w.Code != 409 {
		t.Fatal("another game completed this challenge")
	}
	for _, body := range []string{
		`{"phase":"pending"}`,
		`{"phase":"success","errorCode":"80004005"}`,
		`{"phase":"failed","errorCode":"invalid"}`,
		`{"phase":"success","gamertag":"private"}`,
		`{"phase":"success"}{}`,
	} {
		if w := result(s, body); w.Code != 400 {
			t.Fatalf("invalid result accepted: %s", body)
		}
	}
	// Both games can share a Wine prefix. A legacy shared file must not
	// complete a challenge whose client supplies per-process result events.
	if err := os.WriteFile(s.statusPath, []byte("Signed in as another game\r\n"), 0600); err != nil {
		t.Fatal(err)
	}
	select {
	case <-events:
		t.Fatal("shared Wine status file was used for a result-event client")
	case <-time.After(600 * time.Millisecond):
	}
	if w := result(s, `{"phase":"failed","errorCode":"8abcdef0"}`); w.Code != 200 {
		t.Fatal(w.Code)
	}
	state := <-events
	if state.Phase != "failed" || state.ErrorCode != "8ABCDEF0" || state.UserCode != "" || state.Revision <= pending.Revision {
		t.Fatalf("result=%+v", state)
	}
	if w := result(s, `{"phase":"success"}`); w.Code != 409 {
		t.Fatal("completed challenge accepted a second result")
	}
}
