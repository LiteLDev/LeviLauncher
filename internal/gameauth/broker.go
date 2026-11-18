// Package gameauth transports device-code prompts, never account credentials,
// between the optional game client and the native launcher.
package gameauth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

const Event = "minecraft.account.signin"

type State struct {
	Revision        uint64 `json:"revision"`
	RequestID       string `json:"requestID"`
	Instance        string `json:"instance"`
	Phase           string `json:"phase"`
	UserCode        string `json:"userCode"`
	VerificationURL string `json:"verificationURL"`
	ExpiresAt       int64  `json:"expiresAt"`
	ErrorCode       string `json:"errorCode"`
	BrowserError    bool   `json:"browserError"`
}

type Broker struct {
	mu       sync.Mutex
	sessions map[string]*Session
	state    State
	server   *http.Server
	address  string
	notify   func(State)
	openURL  func(string) error
	revision uint64
}

type Session struct {
	broker                      *Broker
	id, token, name, statusPath string
	ctx                         context.Context
	cancel                      context.CancelFunc
	requestID                   string
}

var codePattern = regexp.MustCompile(`^[A-Z0-9]{8}$`)
var errorPattern = regexp.MustCompile(`^[a-fA-F0-9]{8}$`)
var failurePattern = regexp.MustCompile(`^Sign-in did not complete \(hr=([a-fA-F0-9]{8})\)`)
var defaultMu sync.RWMutex
var defaultBroker *Broker

func randomID() string {
	var data [24]byte
	if _, err := rand.Read(data[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(data[:])
}

func Start(notify func(State), openURL func(string) error) (*Broker, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	b := &Broker{sessions: make(map[string]*Session), state: State{Phase: "idle"}, address: listener.Addr().String(), notify: notify, openURL: openURL}
	b.server = &http.Server{Handler: b, ReadHeaderTimeout: 2 * time.Second, ReadTimeout: 3 * time.Second, WriteTimeout: 3 * time.Second, MaxHeaderBytes: 4096}
	defaultMu.Lock()
	defaultBroker = b
	defaultMu.Unlock()
	go func() { _ = b.server.Serve(listener) }()
	return b, nil
}

func (b *Broker) Close() {
	defaultMu.Lock()
	if defaultBroker == b {
		defaultBroker = nil
	}
	defaultMu.Unlock()
	_ = b.server.Close()
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, s := range b.sessions {
		s.cancel()
	}
	b.sessions = make(map[string]*Session)
}

func CurrentState() State {
	defaultMu.RLock()
	b := defaultBroker
	defaultMu.RUnlock()
	if b == nil {
		return State{Phase: "idle"}
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.state
}

// Register has no effect on Windows: only the native main starts the broker.
func Register(instance, prefix string) *Session {
	defaultMu.RLock()
	b := defaultBroker
	defaultMu.RUnlock()
	if b == nil {
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	s := &Session{broker: b, id: randomID(), token: randomID(), name: instance, statusPath: filepath.Join(prefix, "drive_c", "xgr_signin.txt"), ctx: ctx, cancel: cancel}
	b.mu.Lock()
	b.sessions[s.id] = s
	b.mu.Unlock()
	return s
}

func (s *Session) Environment() map[string]string {
	return map[string]string{"LEVI_GAME_AUTH_URL": "http://" + s.broker.address + "/v1/game-login/" + s.id, "LEVI_GAME_AUTH_TOKEN": s.token}
}

func (s *Session) Close() {
	s.cancel()
	b := s.broker
	b.mu.Lock()
	delete(b.sessions, s.id)
	state := b.state
	changed := state.RequestID == s.requestID && state.Phase == "pending"
	if changed {
		state.Phase = "closed"
		state.UserCode = ""
		state = b.updateLocked(state)
	}
	b.mu.Unlock()
	if changed {
		b.emit(state)
	}
}

func (b *Broker) emit(state State) {
	if b.notify != nil {
		b.notify(state)
	}
}

func (b *Broker) updateLocked(state State) State {
	b.revision++
	state.Revision = b.revision
	b.state = state
	return state
}

func (b *Broker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || r.Host != b.address {
		http.Error(w, "not found", 404)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/v1/game-login/")
	result := strings.HasSuffix(id, "/result")
	if result {
		id = strings.TrimSuffix(id, "/result")
	}
	b.mu.Lock()
	s := b.sessions[id]
	b.mu.Unlock()
	if s == nil || subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+s.token)) != 1 {
		http.Error(w, "unauthorized", 401)
		return
	}
	if r.Header.Get("Content-Type") != "application/json" {
		http.Error(w, "invalid content type", 415)
		return
	}
	if result {
		b.receiveResult(w, r, s)
		return
	}
	var request struct {
		Code         string `json:"code"`
		ExpiresIn    int    `json:"expiresIn"`
		ResultEvents bool   `json:"resultEvents"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2048))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&request) != nil || !codePattern.MatchString(request.Code) || request.ExpiresIn < 1 || request.ExpiresIn > 900 {
		http.Error(w, "invalid challenge", 400)
		return
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		http.Error(w, "invalid challenge", 400)
		return
	}
	observed := time.Now()
	if info, err := os.Stat(s.statusPath); err == nil {
		observed = info.ModTime()
	}
	b.mu.Lock()
	if s.ctx.Err() != nil {
		b.mu.Unlock()
		http.Error(w, "session ended", 410)
		return
	}
	if b.state.RequestID == s.requestID && b.state.Phase == "pending" && b.state.UserCode == request.Code {
		b.mu.Unlock()
		w.WriteHeader(200)
		return
	}
	state := State{RequestID: randomID(), Instance: s.name, Phase: "pending", UserCode: request.Code, VerificationURL: "https://www.microsoft.com/link", ExpiresAt: time.Now().Add(time.Duration(request.ExpiresIn) * time.Second).Unix()}
	s.requestID = state.RequestID
	state = b.updateLocked(state)
	b.mu.Unlock()
	b.emit(state)
	w.Header().Set("Content-Type", "application/json")
	_, _ = io.WriteString(w, `{"accepted":true}`)
	go b.follow(s, state, observed, request.ResultEvents)
}

func (b *Broker) receiveResult(w http.ResponseWriter, r *http.Request, s *Session) {
	var result struct {
		Phase     string `json:"phase"`
		ErrorCode string `json:"errorCode"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2048))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&result) != nil || decoder.Decode(&struct{}{}) != io.EOF ||
		(result.Phase != "success" && result.Phase != "failed") ||
		(result.Phase == "success" && result.ErrorCode != "") ||
		(result.Phase == "failed" && !errorPattern.MatchString(result.ErrorCode)) {
		http.Error(w, "invalid result", 400)
		return
	}
	b.mu.Lock()
	if s.ctx.Err() != nil || s.requestID == "" || b.state.RequestID != s.requestID || b.state.Phase != "pending" {
		b.mu.Unlock()
		http.Error(w, "no pending challenge", 409)
		return
	}
	state := b.state
	state.Phase = result.Phase
	state.ErrorCode = strings.ToUpper(result.ErrorCode)
	state.UserCode = ""
	state = b.updateLocked(state)
	b.mu.Unlock()
	b.emit(state)
	w.WriteHeader(http.StatusOK)
}

func (b *Broker) follow(s *Session, state State, observed time.Time, resultEvents bool) {
	if s.ctx.Err() != nil {
		return
	}
	if b.openURL != nil {
		if err := b.openURL(state.VerificationURL + "?otc=" + url.QueryEscape(state.UserCode)); err != nil {
			b.mu.Lock()
			changed := b.state.RequestID == state.RequestID && b.state.Phase == "pending"
			if changed {
				state = b.state
				state.BrowserError = true
				state = b.updateLocked(state)
			}
			b.mu.Unlock()
			if changed {
				b.emit(state)
			}
		}
	}
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
		}
		b.mu.Lock()
		current := b.state
		b.mu.Unlock()
		if current.RequestID != state.RequestID || current.Phase != "pending" {
			return
		}
		phase, code := "", ""
		if time.Now().Unix() >= state.ExpiresAt {
			phase = "expired"
		} else if info, err := os.Stat(s.statusPath); !resultEvents && err == nil && info.ModTime().After(observed) && info.Size() <= 2048 {
			data, err := os.ReadFile(s.statusPath)
			if err == nil {
				phase, code = terminalStatus(string(data))
			}
		}
		if phase == "" {
			continue
		}
		b.mu.Lock()
		if b.state.RequestID != state.RequestID || b.state.Phase != "pending" {
			b.mu.Unlock()
			return
		}
		state = b.state
		state.Phase = phase
		state.ErrorCode = code
		state.UserCode = ""
		state = b.updateLocked(state)
		b.mu.Unlock()
		b.emit(state)
		return
	}
}

func terminalStatus(text string) (string, string) {
	if strings.HasPrefix(text, "Signed in as ") {
		return "success", ""
	}
	if match := failurePattern.FindStringSubmatch(text); match != nil {
		return "failed", strings.ToUpper(match[1])
	}
	return "", ""
}
