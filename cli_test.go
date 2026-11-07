package main

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestIsCLIInvocation(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want bool
	}{
		{nil, false},
		{[]string{"--debug"}, false},
		{[]string{"debug"}, false},
		{[]string{"/debug"}, false},
		{[]string{"--launch=1.21"}, false},
		{[]string{"--self-update=1.0.2"}, false},
		{[]string{"--post-update-restart"}, false},
		{[]string{"list"}, true},
		{[]string{"launch", "x"}, true},
		{[]string{"call"}, true},
		{[]string{"unknown"}, true},
		{[]string{"--help"}, true},
		{[]string{"/?"}, true},
	} {
		if got := isCLIInvocation(tc.args); got != tc.want {
			t.Errorf("isCLIInvocation(%q) = %v, want %v", tc.args, got, tc.want)
		}
	}
}

type fakeService struct{ got []any }

func (s *fakeService) Mixed(ctx context.Context, name string, n int, on bool, list []string) string {
	s.got = []any{ctx != nil, name, n, on, list}
	return "ok"
}
func (s *fakeService) Fail() error       { return errors.New("boom") }
func (s *fakeService) Code() string      { return "ERR_SOMETHING" }
func (s *fakeService) StartupEssential() {}

func TestCLICall(t *testing.T) {
	svc := &fakeService{}
	methods := collectCLIMethods(svc)
	if _, ok := methods["StartupEssential"]; ok {
		t.Fatal("hidden method is listed")
	}
	m, ok := lookupCLIMethod(methods, "fakeservice.mixed")
	if !ok {
		t.Fatal("method lookup by Service.Method failed")
	}
	if _, ok := lookupCLIMethod(methods, "Other.Mixed"); ok {
		t.Fatal("lookup ignored the service name")
	}
	if got, want := m.signature(), "fakeService.Mixed(string, int, bool, []string) string"; got != want {
		t.Fatalf("signature = %q, want %q", got, want)
	}

	in, err := m.buildArgs(context.Background(), []string{`C:\a b`, "42", "true", `["x","y"]`})
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	o := &cliOutput{stdout: &stdout, stderr: &stderr}
	if o.printResult(m.fn.Call(in)) {
		t.Fatal("successful call reported as failed")
	}
	want := []any{true, `C:\a b`, 42, true, []string{"x", "y"}}
	if !reflect.DeepEqual(svc.got, want) {
		t.Fatalf("arguments = %#v, want %#v", svc.got, want)
	}
	if stdout.String() != "ok\n" {
		t.Fatalf("stdout = %q", stdout.String())
	}

	if _, err := m.buildArgs(context.Background(), []string{"a", "notanint", "true", "[]"}); err == nil {
		t.Fatal("bad JSON accepted")
	}
	if _, err := m.buildArgs(context.Background(), []string{"a"}); err == nil {
		t.Fatal("missing arguments accepted")
	}
	if _, err := m.buildArgs(context.Background(), []string{"a", "1", "true", "[]", "extra"}); err == nil {
		t.Fatal("extra arguments accepted")
	}

	for _, name := range []string{"Fail", "Code"} {
		stderr.Reset()
		m, _ := lookupCLIMethod(methods, name)
		if !o.printResult(m.fn.Call(nil)) {
			t.Fatalf("%s not reported as failed", name)
		}
		if !strings.HasPrefix(stderr.String(), "error: ") {
			t.Fatalf("%s stderr = %q", name, stderr.String())
		}
	}
}
