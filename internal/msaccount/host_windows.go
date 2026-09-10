//go:build windows

package msaccount

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"unsafe"

	"github.com/liteldev/LeviLauncher/internal/nativeinstall"
	"github.com/wailsapp/go-webview2/pkg/edge"
	"github.com/wailsapp/go-webview2/webviewloader"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"golang.org/x/sys/windows"
)

var ErrClosed = errors.New("Microsoft login window closed")

type promptResult struct {
	data map[string]string
	err  error
}

// Host uses a Wails-managed window as its native container. Its remote WebView2
// controller has no Wails runtime, asset handler, services or host objects. The
// Wails controller loads only a static blank document throughout its lifetime.
// All COM/controller state below is confined to Wails' UI thread.
type Host struct {
	window                   *application.WebviewWindow
	contentWindow            windows.Handle
	controller               *edge.ICoreWebView2Controller
	view                     *edge.ICoreWebView2
	profile, browserPath     string
	nonce, nextURL, scriptID string
	closed                   bool
	ready                    chan struct{}
	readyOnce                sync.Once
	results                  chan promptResult
}

func NewHost(cacheDir, browserPath string, onClosed func()) (*Host, error) {
	profile, err := os.MkdirTemp(cacheDir, "login-session-")
	if err != nil {
		return nil, err
	}
	h := &Host{profile: profile, browserPath: browserPath, ready: make(chan struct{}), results: make(chan promptResult, 1)}
	h.window = application.NewWindow(application.WebviewWindowOptions{
		Name: "microsoft-login", Title: "LeviLauncher — Microsoft (login.live.com)",
		Width: 640, Height: 640, MinWidth: 480, MinHeight: 480,
		HTML:                       `<!doctype html><html><head><meta http-equiv="Content-Security-Policy" content="default-src 'none'"></head><body></body></html>`,
		BackgroundColour:           application.NewRGB(255, 255, 255),
		DefaultContextMenuDisabled: true,
	})
	h.window.RegisterHook(events.Common.WindowClosing, func(_ *application.WindowEvent) {
		application.InvokeSync(func() {
			if !h.closed && onClosed != nil {
				onClosed()
			}
			h.shutdown()
			h.deliver(promptResult{err: ErrClosed})
		})
	})
	h.window.OnWindowEvent(events.Common.WindowDidResize, func(_ *application.WindowEvent) { application.InvokeSync(h.resize) })
	h.window.OnWindowEvent(events.Windows.WebViewNavigationCompleted, func(_ *application.WindowEvent) { application.InvokeSync(h.resize) })
	h.window.OnWindowEvent(events.Common.WindowFocus, func(_ *application.WindowEvent) {
		application.InvokeAsync(func() {
			if !h.closed && h.controller != nil {
				h.resize()
				_ = h.controller.MoveFocus(0)
			}
		})
	})
	application.Get().Window.Add(h.window)
	h.window.Run()
	application.InvokeSync(func() {
		// Isolate the guest's z-order from Wails' blank controller, which is
		// shown again when its own navigation and resize callbacks complete.
		class, _ := windows.UTF16PtrFromString("STATIC")
		hwnd, _, _ := windows.NewLazySystemDLL("user32.dll").NewProc("CreateWindowExW").Call(
			0, uintptr(unsafe.Pointer(class)), 0, 0x56000000,
			0, 0, 1, 1, uintptr(h.window.NativeWindow()), 0, 0, 0)
		if hwnd == 0 {
			h.fail()
			return
		}
		h.contentWindow = windows.Handle(hwnd)
		if err := webviewloader.CreateCoreWebView2EnvironmentWithOptions(h,
			webviewloader.WithUserDataFolder(profile), webviewloader.WithBrowserExecutableFolder(browserPath),
			webviewloader.WithAllowSingleSignOnUsingOSPrimaryAccount(false),
			webviewloader.WithExclusiveUserDataFolderAccess(true)); err != nil {
			h.fail()
		}
	})
	return h, nil
}

func (h *Host) EnvironmentCompleted(code webviewloader.HRESULT, env *webviewloader.ICoreWebView2Environment) webviewloader.HRESULT {
	if h.closed {
		return 0
	}
	if code < 0 || env == nil {
		h.fail()
		return 0
	}
	err := withCallback[controllerCallback](func(code uintptr, pointer unsafe.Pointer) {
		if int32(code) < 0 || pointer == nil {
			h.fail()
			return
		}
		controller := (*edge.ICoreWebView2Controller)(pointer)
		if h.closed {
			_ = comCall(pointer, 24)
			return
		}
		controller.AddRef()
		h.controller = controller
		view, err := controller.GetCoreWebView2()
		if err != nil {
			h.fail()
			return
		}
		h.view = view
		if h.configure() != nil {
			h.fail()
			return
		}
		h.resize()
		h.readyOnce.Do(func() { close(h.ready) })
	}, func(ref uintptr) error { return comCall(unsafe.Pointer(env), 3, uintptr(h.contentWindow), ref) })
	if err != nil {
		h.fail()
	}
	return 0
}

func (h *Host) configure() error {
	settings, err := h.view.GetSettings()
	if err != nil {
		return err
	}
	defer comCall(unsafe.Pointer(settings), 2)
	for _, set := range []func(bool) error{settings.PutAreDevToolsEnabled, settings.PutAreDefaultContextMenusEnabled, settings.PutAreHostObjectsAllowed, settings.PutIsStatusBarEnabled} {
		if err := set(false); err != nil {
			return err
		}
	}
	s2 := settings.GetICoreWebView2Settings2()
	if s2 == nil {
		return errors.New("WebView2 user agent interface unavailable")
	}
	defer comCall(unsafe.Pointer(s2), 2)
	if err := s2.PutUserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64; MSAppHost/3.0) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/70.0.3538.102 Safari/537.36 Edge/18.26100"); err != nil {
		return err
	}
	p := unsafe.Pointer(h.view)
	if err := registerCallback[messageCallback](p, 34, h.message); err != nil {
		return err
	}
	if err := registerCallback[resourceCallback](p, 55, h.request); err != nil {
		return err
	}
	filter, _ := windows.UTF16PtrFromString("https://login.live.com/*")
	if err := comCall(p, 57, uintptr(unsafe.Pointer(filter)), uintptr(edge.COREWEBVIEW2_WEB_RESOURCE_CONTEXT_DOCUMENT)); err != nil {
		return err
	}
	if err := registerCallback[navigationCallback](p, 15, func(_ uintptr, _ unsafe.Pointer) {
		if h.closed || h.nonce == "" {
			return
		}
		source, err := h.view.GetSource()
		if err != nil || !trustedSource(source) {
			return
		}
		u, _ := url.Parse(source)
		if u.Path == "/ppsecure/post.srf" {
			h.eval(`if(typeof ServerData==='object'){window.external.notify(JSON.stringify(ServerData));}`)
		}
	}); err != nil {
		return err
	}
	if err := registerCallback[navigationStartingCallback](p, 7, func(_ uintptr, args unsafe.Pointer) {
		var uri *uint16
		if comCall(args, 3, uintptr(unsafe.Pointer(&uri))) != nil {
			_ = comCall(args, 8, 1)
			return
		}
		raw := windows.UTF16PtrToString(uri)
		windows.CoTaskMemFree(unsafe.Pointer(uri))
		u, err := url.Parse(raw)
		// Microsoft authentication can navigate to its account/MFA domains, but
		// local assets, files, custom protocols and embedded credentials are denied.
		if err != nil || u.Scheme != "https" || u.User != nil || (u.Port() != "" && u.Port() != "443") || !microsoftHost(u.Hostname()) {
			_ = comCall(args, 8, 1)
		}
	}); err != nil {
		return err
	}
	if err := registerCallback[newWindowCallback](p, 44, func(_ uintptr, args unsafe.Pointer) { _ = comCall(args, 6, 1) }); err != nil {
		return err
	}
	if err := registerCallback[permissionCallback](p, 23, func(_ uintptr, args unsafe.Pointer) { _ = comCall(args, 7, 2) }); err != nil {
		return err
	}
	return registerCallback[processCallback](p, 25, func(_ uintptr, _ unsafe.Pointer) { h.fail() })
}

func microsoftHost(host string) bool {
	for _, allowed := range []string{"login.live.com", "account.live.com", "account.microsoft.com", "login.microsoftonline.com"} {
		if host == allowed {
			return true
		}
	}
	return false
}

func (h *Host) Prompt(ctx context.Context, next string) (map[string]string, error) {
	if !nativeinstall.IsLoginURL(next) {
		return nil, errors.New("untrusted login continuation")
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case r := <-h.results:
		return r.data, r.err
	case <-h.ready:
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, err
	}
	application.InvokeSync(func() {
		if h.closed {
			h.deliver(promptResult{err: ErrClosed})
			return
		}
		h.nonce = hex.EncodeToString(nonce[:])
		h.nextURL = next
		if h.scriptID != "" {
			id, _ := windows.UTF16PtrFromString(h.scriptID)
			if comCall(unsafe.Pointer(h.view), 28, uintptr(unsafe.Pointer(id))) != nil {
				h.fail()
				return
			}
			h.scriptID = ""
		}
		encoded, _ := json.Marshal(h.nonce)
		script := `if(location.origin==='https://login.live.com'&&window.top===window){const notify=(d)=>chrome.webview.postMessage(JSON.stringify({Nonce:` + string(encoded) + `,Payload:typeof d==='string'?d:JSON.stringify(d)}));Object.defineProperty(window,'external',{value:{notify},configurable:true});}`
		scriptPtr, _ := windows.UTF16PtrFromString(script)
		err := withCallback[scriptCallback](func(code uintptr, id unsafe.Pointer) {
			if h.closed {
				return
			}
			if int32(code) < 0 {
				h.fail()
				return
			}
			h.scriptID = windows.UTF16PtrToString((*uint16)(id))
			if h.view.Navigate(h.nextURL) != nil {
				h.fail()
			}
		}, func(ref uintptr) error {
			return comCall(unsafe.Pointer(h.view), 27, uintptr(unsafe.Pointer(scriptPtr)), ref)
		})
		runtime.KeepAlive(scriptPtr)
		if err != nil {
			h.fail()
		}
	})
	h.Focus()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case r := <-h.results:
		return r.data, r.err
	}
}

func (h *Host) message(_ uintptr, pointer unsafe.Pointer) {
	if h.closed || h.nonce == "" {
		return
	}
	args := (*edge.ICoreWebView2WebMessageReceivedEventArgs)(pointer)
	source, err := args.GetSource()
	if err != nil {
		return
	}
	top, err := h.view.GetSource()
	if err != nil {
		return
	}
	raw, err := args.TryGetWebMessageAsString()
	if err != nil {
		return
	}
	data, reply := loginResult(raw, source, top, h.nonce)
	if data != nil {
		h.nonce = ""
		h.deliver(promptResult{data: data})
		return
	}
	if reply != "" {
		h.eval(reply)
	}
}

func (h *Host) request(_ uintptr, pointer unsafe.Pointer) {
	if h.closed {
		return
	}
	args := (*edge.ICoreWebView2WebResourceRequestedEventArgs)(pointer)
	request, err := args.GetRequest()
	if err != nil {
		h.fail()
		return
	}
	defer request.Release()
	uri, err := request.GetUri()
	if err != nil || !trustedSource(uri) {
		return
	}
	headers, err := request.GetHeaders()
	if err != nil {
		h.fail()
		return
	}
	defer headers.Release()
	for key, value := range hostHeaders(h.nonce) {
		if headers.SetHeader(key, value) != nil {
			h.fail()
			return
		}
	}
}

func (h *Host) eval(script string) {
	if !h.closed && h.view != nil {
		if h.view.ExecuteScript(script, nil) != nil {
			h.fail()
		}
	}
}
func (h *Host) deliver(r promptResult) {
	select {
	case h.results <- r:
	default:
	}
}
func (h *Host) fail() { h.deliver(promptResult{err: errors.New("Microsoft login WebView2 failed")}) }
func (h *Host) resize() {
	if h.closed || h.controller == nil {
		return
	}
	var rect struct{ Left, Top, Right, Bottom int32 }
	ok, _, _ := windows.NewLazySystemDLL("user32.dll").NewProc("GetClientRect").Call(uintptr(h.window.NativeWindow()), uintptr(unsafe.Pointer(&rect)))
	if ok != 0 {
		// HWND_TOP without activation keeps resizing from stealing focus.
		_, _, _ = windows.NewLazySystemDLL("user32.dll").NewProc("SetWindowPos").Call(
			uintptr(h.contentWindow), 0, 0, 0, uintptr(rect.Right), uintptr(rect.Bottom), 0x0010)
		_ = comCall(unsafe.Pointer(h.controller), 6, uintptr(unsafe.Pointer(&rect)))
	}
}
func (h *Host) Focus() {
	h.window.Focus()
	application.InvokeSync(func() {
		if !h.closed && h.controller != nil {
			_ = h.controller.MoveFocus(0)
		}
	})
}
func (h *Host) shutdown() {
	if h.closed {
		return
	}
	h.closed = true
	h.nonce = ""
	if h.controller != nil {
		_ = comCall(unsafe.Pointer(h.controller), 24)
	}
	if h.view != nil {
		h.view.Release()
		h.view = nil
	}
	if h.controller != nil {
		h.controller.Release()
		h.controller = nil
	}
	if h.contentWindow != 0 {
		_, _, _ = windows.NewLazySystemDLL("user32.dll").NewProc("DestroyWindow").Call(uintptr(h.contentWindow))
		h.contentWindow = 0
	}
}
func (h *Host) Close() {
	application.InvokeSync(h.shutdown)
	h.window.Close()
	// An exclusive per-attempt profile prevents silent account reuse. WebView2
	// may release file locks asynchronously; cleanup retries at next startup.
	_ = os.RemoveAll(h.profile)
}

func CleanProfiles(cacheDir string) {
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() && len(entry.Name()) > 14 && entry.Name()[:14] == "login-session-" {
			_ = os.RemoveAll(filepath.Join(cacheDir, entry.Name()))
		}
	}
}
