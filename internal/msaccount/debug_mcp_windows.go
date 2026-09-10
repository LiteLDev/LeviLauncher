//go:build windows && mcp

package msaccount

import (
	"encoding/json"
	"net/url"
	"time"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"
	"golang.org/x/sys/windows"
)

// DebugSnapshot is compiled only into explicit MCP inspection builds. Never
// return URL queries, DOM text, login messages, cookies or credentials.
func (h *Host) DebugSnapshot() map[string]any {
	return application.InvokeSyncWithResult(func() map[string]any {
		r := map[string]any{"closed": h.closed, "controller": h.controller != nil, "view": h.view != nil, "awaitingCallback": h.nonce != "", "scriptInstalled": h.scriptID != ""}
		select {
		case <-h.ready:
			r["ready"] = true
		default:
			r["ready"] = false
		}
		if h.controller != nil {
			var visible int32
			r["visibilityError"] = comCall(unsafe.Pointer(h.controller), 3, uintptr(unsafe.Pointer(&visible))) != nil
			r["visible"] = visible != 0
			if b, err := h.controller.GetBounds(); err == nil {
				r["bounds"] = map[string]int32{"left": b.Left, "top": b.Top, "right": b.Right, "bottom": b.Bottom}
			}
		}
		if h.view != nil {
			if raw, err := h.view.GetSource(); err == nil {
				if u, err := url.Parse(raw); err == nil {
					r["origin"] = u.Scheme + "://" + u.Host
					r["path"] = u.Path
				}
			}
		}
		return r
	})
}

func (h *Host) DebugPageMetrics() map[string]any {
	result := make(chan map[string]any, 1)
	application.InvokeAsync(func() {
		if h.closed || h.view == nil {
			result <- map[string]any{"unavailable": true}
			return
		}
		script, _ := windows.UTF16PtrFromString(`JSON.stringify({ready:document.readyState,children:document.body?.childElementCount,textLength:document.body?.innerText.length,inputs:document.querySelectorAll('input').length,background:document.body?getComputedStyle(document.body).backgroundColor:'',externalNotify:typeof window.external?.notify})`)
		err := withCallback[executeCallback](func(code uintptr, p unsafe.Pointer) {
			r := map[string]any{"failed": int32(code) < 0}
			if p != nil {
				var nested string
				if json.Unmarshal([]byte(windows.UTF16PtrToString((*uint16)(p))), &nested) == nil {
					_ = json.Unmarshal([]byte(nested), &r)
				}
			}
			select {
			case result <- r:
			default:
			}
		}, func(ref uintptr) error {
			return comCall(unsafe.Pointer(h.view), 29, uintptr(unsafe.Pointer(script)), ref)
		})
		if err != nil {
			result <- map[string]any{"failed": true}
		}
	})
	select {
	case r := <-result:
		return r
	case <-time.After(5 * time.Second):
		return map[string]any{"timeout": true}
	}
}
