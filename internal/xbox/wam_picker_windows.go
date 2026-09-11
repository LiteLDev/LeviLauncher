//go:build windows

package xbox

import (
	"context"
	"fmt"
	"time"
	"unsafe"

	"github.com/go-ole/go-ole"
)

const (
	wamPaneClass      = "Windows.UI.ApplicationSettings.AccountsSettingsPane"
	wamPaneIID        = "81ea942c-4f09-4406-a538-838d9b14b7e6"
	wamPaneInteropIID = "d3ee12ad-3865-4362-9746-b75a682df0e6"
	wamPaneEventIID   = "69b8847e-7d72-5a15-bc1c-4ca39c93b162"
	wamProviderCmdIID = "b7de5527-4c8f-42dd-84da-5ec493abdb9a"
	wamAsyncActionIID = "5a648006-843a-4da9-865b-9d26e5dfad7b"
)

// All pane objects, callbacks and state are confined to the HWND's UI thread.
// dispatch must execute synchronously on that thread without blocking its pump
// for the lifetime of the dialog. The caller runs on a WinRT-initialized worker
// and owns the returned provider reference, including its native selection state.
func selectWAMProvider(ctx context.Context, hwnd uintptr, dispatch func(func())) (*ole.IUnknown, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if hwnd == 0 || dispatch == nil {
		return nil, ErrAuthenticationFailed
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	provider, err := findMSAProvider(ctx)
	if err != nil {
		return nil, err
	}
	defer provider.Release()
	p := &wamAccountPane{}
	dispatch(func() { p.err = p.open(hwnd, provider) })
	defer dispatch(p.close)
	ticker := time.NewTicker(40 * time.Millisecond)
	defer ticker.Stop()
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		var done bool
		var selected *ole.IUnknown
		var resultErr error
		dispatch(func() {
			defer func() {
				if done {
					resultErr = p.err
					if resultErr == nil {
						selected = p.takeSelection()
					}
					p.close()
				}
			}()
			if p.err != nil {
				done = true
				return
			}
			var status int32
			p.err = wamCall(p.info, 7, uintptr(unsafe.Pointer(&status)))
			done = p.err != nil || status != asyncStarted
			if p.err == nil && done {
				if status == 2 { // AsyncStatus.Canceled
					p.err = context.Canceled
				} else {
					p.err = wamCall(p.action, 8) // IAsyncAction.GetResults (no result pointer)
				}
			}
		})
		if done {
			if resultErr != nil {
				return nil, resultErr
			}
			if err := ctx.Err(); err != nil {
				if selected != nil {
					selected.Release()
				}
				return nil, err
			}
			if selected == nil {
				return nil, context.Canceled
			}
			return selected, nil
		}
		select {
		case <-ctx.Done():
		case <-ticker.C:
		}
	}
}

type wamAccountPane struct {
	pane, event, action, info *ole.IUnknown
	cookie                    int64
	subscribed, closed        bool
	initialized               bool
	selected                  *ole.IUnknown
	err                       error
}

func (p *wamAccountPane) open(hwnd uintptr, provider *ole.IUnknown) error {
	if err := initializeWAM(0); err != nil { // RO_INIT_SINGLETHREADED on the HWND owner thread.
		return err
	}
	p.initialized = true
	interop, err := ole.RoGetActivationFactory(wamPaneClass, ole.NewGUID(wamPaneInteropIID))
	if err != nil {
		return err
	}
	defer interop.Release()
	if err := wamCall(&interop.IUnknown, 6, hwnd, uintptr(unsafe.Pointer(ole.NewGUID(wamPaneIID))), uintptr(unsafe.Pointer(&p.pane))); err != nil {
		return err
	}
	p.event = newWAMDelegate(wamPaneEventIID, false, func(_, args unsafe.Pointer) uintptr {
		if !p.closed && p.err == nil {
			p.err = p.populate((*ole.IUnknown)(args), provider)
		}
		return ole.S_OK
	})
	if err := wamCall(p.pane, 6, uintptr(unsafe.Pointer(p.event)), uintptr(unsafe.Pointer(&p.cookie))); err != nil {
		return err
	}
	p.subscribed = true
	// Desktop equivalent of AccountsSettingsPane.ShowAddAccountAsync.
	if err := wamCall(&interop.IUnknown, 8, hwnd, uintptr(unsafe.Pointer(ole.NewGUID(wamAsyncActionIID))), uintptr(unsafe.Pointer(&p.action))); err != nil {
		return err
	}
	if p.action == nil {
		return ErrAuthenticationFailed
	}
	info, err := p.action.QueryInterface(ole.NewGUID(iidIAsyncInfo))
	if err != nil {
		return err
	}
	p.info = &info.IUnknown
	return p.err
}

func (p *wamAccountPane) populate(args, provider *ole.IUnknown) (err error) {
	deferral, err := wamObject(args, 12)
	if err != nil {
		return fmt.Errorf("account panel deferral: %w", err)
	}
	defer deferral.Release()
	defer func() {
		if completeErr := wamCall(deferral, 6); err == nil {
			err = completeErr
		}
	}()
	providers, err := wamObject(args, 6)
	if err != nil {
		return fmt.Errorf("account panel provider commands: %w", err)
	}
	defer providers.Release()
	factory, err := ole.RoGetActivationFactory("Windows.UI.ApplicationSettings.WebAccountProviderCommand", ole.NewGUID("d5658a1b-b176-4776-8469-a9d3ff0b3f59"))
	if err != nil {
		return err
	}
	defer factory.Release()
	invoked := newWAMDelegate(wamProviderCmdIID, true, func(command, _ unsafe.Pointer) uintptr {
		if p.closed || p.err != nil {
			return ole.S_OK
		}
		// The callback provider carries the account chosen by the system UI.
		// Re-finding the generic MSA provider here would lose that selection.
		selected, err := wamObject((*ole.IUnknown)(command), 6)
		if err != nil {
			p.err = err
		} else {
			p.choose(selected)
		}
		return ole.S_OK
	})
	defer invoked.Release()
	var command *ole.IUnknown
	if err := wamCall(&factory.IUnknown, 6, uintptr(unsafe.Pointer(provider)), uintptr(unsafe.Pointer(invoked)), uintptr(unsafe.Pointer(&command))); err != nil {
		return err
	}
	if command == nil {
		return ErrAuthenticationFailed
	}
	defer command.Release()
	return wamCall(providers, 13, uintptr(unsafe.Pointer(command))) // IVector.Append
}

func (p *wamAccountPane) choose(provider *ole.IUnknown) {
	if p.closed || p.selected != nil || p.err != nil {
		provider.Release()
		return
	}
	p.selected = provider
}

func (p *wamAccountPane) takeSelection() *ole.IUnknown {
	provider := p.selected
	p.selected = nil
	return provider
}

func (p *wamAccountPane) close() {
	if p.closed {
		return
	}
	p.closed = true
	if p.initialized {
		defer uninitializeWAM()
	}
	if p.subscribed {
		// EventRegistrationToken is passed by value (64 bits).
		if unsafe.Sizeof(uintptr(0)) == 4 {
			_ = wamCall(p.pane, 7, uintptr(uint32(p.cookie)), uintptr(uint64(p.cookie)>>32))
		} else {
			_ = wamCall(p.pane, 7, uintptr(p.cookie))
		}
	}
	if p.info != nil {
		_ = wamCall(p.info, 9) // Cancel any still-open native pane.
		p.info.Release()
	}
	for _, object := range []*ole.IUnknown{p.action, p.event, p.pane, p.selected} {
		if object != nil {
			object.Release()
		}
	}
	p.pane, p.event, p.action, p.info, p.selected = nil, nil, nil, nil, nil
}
