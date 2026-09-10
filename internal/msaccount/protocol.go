// SPDX-License-Identifier: GPL-3.0-only
// InlineLogin host protocol: Xodus 0670e25a, nativeinstall/THIRD_PARTY_NOTICES.
package msaccount

import (
	"encoding/json"
	"net/url"
	"strings"
)

func trustedSource(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.Hostname() == "login.live.com" &&
		(u.Port() == "" || u.Port() == "443") && u.User == nil
}

func loginResult(raw, source, top, nonce string) (map[string]string, string) {
	if nonce == "" || len(raw) > 1<<20 || !trustedSource(source) || !trustedSource(top) {
		return nil, ""
	}
	var envelope struct{ Nonce, Payload string }
	if json.Unmarshal([]byte(raw), &envelope) != nil || envelope.Nonce != nonce {
		return nil, ""
	}
	u, _ := url.Parse(source)
	t, _ := url.Parse(top)
	var fields map[string]json.RawMessage
	if json.Unmarshal([]byte(envelope.Payload), &fields) != nil {
		return nil, ""
	}
	if _, exists := fields["sDAToken"]; exists {
		if u.Path != "/ppsecure/post.srf" || t.Path != u.Path {
			return nil, ""
		}
		selected := map[string]string{}
		for _, key := range []string{"sDAToken", "sDASessionKey", "sDAStartTime", "sDAExpires", "sSTSInlineFlowToken", "sSigninName", "K"} {
			var value string
			if json.Unmarshal(fields[key], &value) != nil {
				return nil, ""
			}
			if value == "" && (key == "sDAToken" || key == "sSigninName" || key == "K") {
				return nil, ""
			}
			selected[key] = value
		}
		if len(selected["sSigninName"]) > 320 || strings.ContainsAny(selected["sSigninName"], "\x00\r\n") || len(selected["K"]) > 128 {
			return nil, ""
		}
		return selected, ""
	}
	var command struct {
		Type  string
		Value struct{ Name, Context string }
	}
	if json.Unmarshal([]byte(envelope.Payload), &command) != nil || command.Type != "invoke" || command.Value.Name != "CloudExperienceHost.getContext" || len(command.Value.Context) > 512 {
		return nil, ""
	}
	response, _ := json.Marshal(map[string]any{"type": "callback", "value": map[string]any{
		"name": "CloudExperienceHost.getContext", "args": []string{"CloudExperienceHost", "TokenBroker", "TokenBroker", capabilities}, "context": command.Value.Context,
	}})
	literal, _ := json.Marshal(string(response))
	return nil, "window['CloudExperienceHost.Bridge.dispatchMessage'](" + string(literal) + ")"
}

const capabilities = `{"PrivatePropertyBag":1,"PasswordlessConnect":1,"PreferAssociate":1,"ChromelessUI":0}`

func hostHeaders(nonce string) map[string]string {
	return map[string]string{
		"cxh-capabilities": capabilities, "cxh-correlationId": nonce,
		"cxh-msaBinaryVersion": "55", "cxh-identityClientBinaryVersion": "3",
		"cxh-osVersionInfo": `{"platformId":2,"majorVersion":10,"minorVersion":0,"buildNumber":26100}`,
		"cxh-platform":      "CloudExperienceHost.Platform.DESKTOP", "cxh-protocol": "TokenBroker", "cxh-source": "TokenBroker", "hostApp": "CloudExperienceHost",
	}
}
