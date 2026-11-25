package deeplink

import "sync"

// Inbox holds the latest link until the frontend takes it. A link can come
// before the window has loaded (the launcher was started by the link) or
// while the frontend cannot navigate (onboarding, an update, a locked page),
// so it is kept rather than only emitted, and a newer link replaces an older
// one that was not taken yet.
type Inbox struct {
	mu      sync.Mutex
	pending *Request
	notify  func(Request)
}

// SetNotify sets the function Deliver calls after storing a link, typically
// one that emits EventOpen to the frontend.
func (b *Inbox) SetNotify(notify func(Request)) {
	b.mu.Lock()
	b.notify = notify
	b.mu.Unlock()
}

// Deliver stores r as the pending link and notifies.
func (b *Inbox) Deliver(r Request) {
	b.mu.Lock()
	b.pending = &r
	notify := b.notify
	b.mu.Unlock()
	if notify != nil {
		notify(r)
	}
}

// DeliverRaw parses raw and delivers it when it is valid.
func (b *Inbox) DeliverRaw(raw string) (Request, error) {
	r, err := Parse(raw)
	if err != nil {
		return Request{}, err
	}
	b.Deliver(r)
	return r, nil
}

// Take returns the pending link and clears it; nil when there is none.
func (b *Inbox) Take() *Request {
	b.mu.Lock()
	defer b.mu.Unlock()
	r := b.pending
	b.pending = nil
	return r
}

// Default is the launcher's inbox.
var Default = &Inbox{}
