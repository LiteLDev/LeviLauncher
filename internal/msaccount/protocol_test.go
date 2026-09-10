package msaccount

import (
	"encoding/json"
	"strings"
	"testing"
)

func loginEnvelope(fields map[string]any, nonce string) string {
	payload, _ := json.Marshal(fields)
	envelope, _ := json.Marshal(map[string]string{"Nonce": nonce, "Payload": string(payload)})
	return string(envelope)
}
func TestSecondStageAllowsEmptyNonCoreFields(t *testing.T) {
	fields := map[string]any{"sDAToken": "<token/>", "sSigninName": "player@example.test", "K": "1234", "sDASessionKey": "", "sDAStartTime": "", "sDAExpires": "", "sSTSInlineFlowToken": ""}
	page := "https://login.live.com/ppsecure/post.srf"
	data, _ := loginResult(loginEnvelope(fields, "current"), page, page, "current")
	if data == nil || data["sSigninName"] != "player@example.test" {
		t.Fatal("second-stage result rejected")
	}
	for _, key := range []string{"sDAToken", "sSigninName", "K"} {
		copy := map[string]any{}
		for k, v := range fields {
			copy[k] = v
		}
		copy[key] = ""
		if data, _ := loginResult(loginEnvelope(copy, "current"), page, page, "current"); data != nil {
			t.Errorf("accepted empty %s", key)
		}
	}
}
func TestLoginRejectsUntrustedAndReplayedMessages(t *testing.T) {
	fields := map[string]any{"sDAToken": "<token/>", "sSigninName": "player@example.test", "K": "1234", "sDASessionKey": "", "sDAStartTime": "", "sDAExpires": "", "sSTSInlineFlowToken": ""}
	page := "https://login.live.com/ppsecure/post.srf"
	for _, tc := range []struct{ source, top, messageNonce, currentNonce string }{
		{"http://login.live.com/ppsecure/post.srf", page, "n", "n"},
		{"https://login.live.com.evil.test/ppsecure/post.srf", page, "n", "n"},
		{"https://login.live.com:444/ppsecure/post.srf", page, "n", "n"},
		{"https://user@login.live.com/ppsecure/post.srf", page, "n", "n"},
		{page, "https://evil.test/ppsecure/post.srf", "n", "n"},
		{page, "https://login.live.com/other", "n", "n"},
		{"https://login.live.com/other", page, "n", "n"},
		{page, page, "old", "current"},
		{page, page, "", ""},
	} {
		if data, reply := loginResult(loginEnvelope(fields, tc.messageNonce), tc.source, tc.top, tc.currentNonce); data != nil || reply != "" {
			t.Fatal("accepted untrusted message")
		}
	}
	fields["sDAExpires"] = 1
	if data, _ := loginResult(loginEnvelope(fields, "n"), page, page, "n"); data != nil {
		t.Fatal("accepted wrong field type")
	}
}
func TestHostContextReplyEscapesUntrustedContext(t *testing.T) {
	page := "https://login.live.com/ppsecure/InlineLogin.srf"
	context := `');evil();//`
	_, reply := loginResult(loginEnvelope(map[string]any{"Type": "invoke", "Value": map[string]string{"Name": "CloudExperienceHost.getContext", "Context": context}}, "n"), page, page, "n")
	if reply == "" || !strings.Contains(reply, `\"context\"`) {
		t.Fatal("context reply must be JSON encoded")
	}
	_, reply = loginResult(loginEnvelope(map[string]any{"Type": "invoke", "Value": map[string]string{"Name": "ReadFile", "Context": "x"}}, "n"), page, page, "n")
	if reply != "" {
		t.Fatal("accepted arbitrary host operation")
	}
}
