#include "xgr.h"
#include <stdlib.h>
// XTaskQueue / XAsync implementation for provider 073b7dcb-1fcf-4030-94be-e3c9eb623428.
// Semantics modeled on libHttpClient Source/Task (TaskQueue.cpp / AsyncLib.cpp).
//
// Slot map (derived from game thunks 0x14E021718..0x14E021E34 and the thunks in
// libHttpClient.GDK.dll / XSAPI C thunks / PlayFabMultiplayerGDK.dll / XCurl.dll):
//   3  HRESULT XAsyncGetStatus(XAsyncBlock*, bool wait)
//   4  HRESULT XAsyncGetResultSize(XAsyncBlock*, size_t* size)
//   5  void    XAsyncCancel(XAsyncBlock*)
//   6  HRESULT XAsyncRun(XAsyncBlock*, XAsyncWork*)                         (by header order; unused)
//   7  HRESULT XAsyncBegin(XAsyncBlock*, void* ctx, const void* identity, const char* name, XAsyncProvider*)
//   8  XAsyncBeginAlloc                                                     (by header order; unused, 7 args)
//   9  HRESULT XAsyncSchedule(XAsyncBlock*, uint32_t delayMs)
//   10 void    XAsyncComplete(XAsyncBlock*, HRESULT result, size_t requiredBufferSize)
//   11 HRESULT XAsyncGetResult(XAsyncBlock*, const void* identity, size_t size, void* buf, size_t* used)
//   12 HRESULT XTaskQueueCreate(mode work, mode completion, XTaskQueueHandle* out)
//   13 HRESULT XTaskQueueCreateComposite(XTaskQueuePortHandle work, XTaskQueuePortHandle compl, XTaskQueueHandle* out)
//   14 HRESULT XTaskQueueGetPort(XTaskQueueHandle, XTaskQueuePort, XTaskQueuePortHandle* out)
//   15 HRESULT XTaskQueueDuplicateHandle(XTaskQueueHandle, XTaskQueueHandle* out)
//   16 bool    XTaskQueueDispatch(XTaskQueueHandle, XTaskQueuePort, uint32_t timeoutMs)
//   17 void    XTaskQueueCloseHandle(XTaskQueueHandle)
//   18 HRESULT XTaskQueueSubmitCallback(XTaskQueueHandle, XTaskQueuePort, void* ctx, XTaskQueueCallback*)
//   19 HRESULT XTaskQueueSubmitDelayedCallback(XTaskQueueHandle, XTaskQueuePort, uint32_t delayMs, void* ctx, XTaskQueueCallback*)
//   20 HRESULT XTaskQueueRegisterWaiter(q, port, HANDLE, ctx, cb, token*)   (by header order; unused)
//   21 void    XTaskQueueUnregisterWaiter(q, token)                         (by header order; unused)
//   22 HRESULT XTaskQueueTerminate(XTaskQueueHandle, bool wait, void* ctx, XTaskQueueTerminatedCallback*)
//   23 HRESULT XTaskQueueRegisterMonitor(XTaskQueueHandle, void* ctx, XTaskQueueMonitorCallback*, XTaskQueueRegistrationToken*)
//   24 void    XTaskQueueUnregisterMonitor(XTaskQueueHandle, XTaskQueueRegistrationToken)
//   25 bool    XTaskQueueGetCurrentProcessTaskQueue(XTaskQueueHandle* out)
//   26 void    XTaskQueueSetCurrentProcessTaskQueue(XTaskQueueHandle)
//   29 void    (no args; called at the top of many thunks) -> no-op

#define TQ_PROV 0x073b7dcbUL
#define E_ILLEGAL_METHOD_CALL_ ((HRESULT)0x8000000EL)
#define E_NOT_SUFFICIENT_BUFFER_ ((HRESULT)0x8007007AL)

typedef void (*XTaskQueueCallback)(void* ctx, unsigned char canceled);
typedef void (*XTaskQueueMonitorCallback)(void* ctx, void* queue, UINT32 port);
typedef void (*XTaskQueueTerminatedCallback)(void* ctx);

enum { MODE_MANUAL = 0, MODE_THREADPOOL = 1, MODE_SERIALIZED = 2, MODE_IMMEDIATE = 3 };
#define QMAGIC 0x31515458UL /* 'XTQ1' */
#define PMAGIC 0x31505458UL /* 'XTP1' */

struct Queue; struct Port;
typedef struct Entry {
  struct Entry* next; XTaskQueueCallback cb; void* ctx;
  struct Queue* q; struct Port* port; UINT32 portIdx; ULONGLONG due;
} Entry;
typedef struct Port {
  DWORD magic; int mode; CRITICAL_SECTION cs; Entry *head, *tail;
  HANDLE avail; /* manual-reset, set while list non-empty (manual mode) */
  LONG busy;    /* serialized mode worker scheduled */
  struct Queue* owner;
} Port;
typedef struct Monitor { struct Monitor* next; UINT64 token; void* ctx; XTaskQueueMonitorCallback cb; } Monitor;
typedef struct Queue {
  DWORD magic; volatile LONG refs; Port* ports[2]; volatile LONG terminated;
  volatile LONG inflight; /* entries submitted via this queue and not yet finished */
  CRITICAL_SECTION cs; Monitor* monitors;
} Queue;

// ---------------------------------------------------------------- globals
static INIT_ONCE g_once = INIT_ONCE_STATIC_INIT;
static CRITICAL_SECTION g_tcs;   // timer list
static Entry* g_timers;          // sorted by due
static HANDLE g_tevent;
static CRITICAL_SECTION g_acs;   // XAsyncBlock internal / process queue
static Queue* g_procQ; static BOOL g_procSet;
static volatile LONG64 g_token;

// per-thread stack of queues whose callbacks are running on this thread
typedef struct Frame { Queue* q; struct Frame* prev; } Frame;
static DWORD g_tls = TLS_OUT_OF_INDEXES;
#define T_FRAMES ((Frame*)TlsGetValue(g_tls))

static DWORD WINAPI timer_thread(LPVOID p);
static BOOL CALLBACK init_fn(PINIT_ONCE o, PVOID p, PVOID* c) {
  g_tls = TlsAlloc();
  InitializeCriticalSection(&g_tcs); InitializeCriticalSection(&g_acs);
  g_tevent = CreateEventW(NULL, FALSE, FALSE, NULL);
  HANDLE t = CreateThread(NULL, 0, timer_thread, NULL, 0, NULL); if (t) CloseHandle(t);
  return TRUE;
}
static void ensure_init(void) { InitOnceExecuteOnce(&g_once, init_fn, NULL, NULL); }

// ---------------------------------------------------------------- queue core
static Port* port_new(int mode, Queue* owner) {
  Port* p = (Port*)calloc(1, sizeof *p); p->magic = PMAGIC; p->mode = mode; p->owner = owner;
  InitializeCriticalSection(&p->cs); p->avail = CreateEventW(NULL, TRUE, FALSE, NULL); return p;
}
static Queue* queue_new(void) {
  Queue* q = (Queue*)calloc(1, sizeof *q); q->magic = QMAGIC; q->refs = 1; InitializeCriticalSection(&q->cs); return q;
}
static Queue* queue_create(int wm, int cm) {
  Queue* q = queue_new(); q->ports[0] = port_new(wm, q); q->ports[1] = port_new(cm, q); return q;
}
static Queue* proc_queue(void) {
  ensure_init(); Queue* q;
  EnterCriticalSection(&g_acs);
  if (!g_procQ && !g_procSet) { g_procQ = queue_create(MODE_THREADPOOL, MODE_THREADPOOL); g_procSet = TRUE; }
  q = g_procQ; LeaveCriticalSection(&g_acs); return q;
}
static Queue* resolve(void* h) {
  ensure_init();
  if (!h) return proc_queue();
  Queue* q = (Queue*)h;
  if (q->magic != QMAGIC) { xgr_log("TQ: invalid queue handle %p", h); return NULL; }
  return q;
}
static BOOL q_dead(Queue* q, Port* p) { return q->terminated || (p->owner && p->owner->terminated); }

static void run_entry(Entry* e, BOOL canceled) {
  Frame f = { e->q, T_FRAMES }; TlsSetValue(g_tls, &f);
  e->cb(e->ctx, canceled ? 1 : 0);
  TlsSetValue(g_tls, f.prev);
  InterlockedDecrement(&e->q->inflight);
  free(e);
}
static Entry* pop(Port* p) {
  EnterCriticalSection(&p->cs);
  Entry* e = p->head;
  if (e) { p->head = e->next; if (!p->head) p->tail = NULL; }
  if (!p->head) ResetEvent(p->avail);
  LeaveCriticalSection(&p->cs); return e;
}
static void notify_monitors(Queue* q, UINT32 portIdx) {
  struct { XTaskQueueMonitorCallback cb; void* ctx; } m[16]; int n = 0;
  EnterCriticalSection(&q->cs);
  for (Monitor* x = q->monitors; x && n < 16; x = x->next) { m[n].cb = x->cb; m[n].ctx = x->ctx; n++; }
  LeaveCriticalSection(&q->cs);
  for (int i = 0; i < n; i++) m[i].cb(m[i].ctx, q, portIdx);
}
static DWORD WINAPI wi_run_entry(LPVOID p) { run_entry((Entry*)p, FALSE); return 0; }
static DWORD WINAPI wi_threadpool(LPVOID pp) { Entry* e = pop((Port*)pp); if (e) run_entry(e, FALSE); return 0; }
static DWORD WINAPI wi_serial(LPVOID pp) {
  Port* p = (Port*)pp;
  for (;;) {
    EnterCriticalSection(&p->cs);
    Entry* e = p->head;
    if (!e) { p->busy = 0; LeaveCriticalSection(&p->cs); return 0; }
    p->head = e->next; if (!p->head) p->tail = NULL;
    LeaveCriticalSection(&p->cs);
    run_entry(e, FALSE);
  }
}
static void append(Port* p, Entry* e) {
  e->next = NULL; if (p->tail) p->tail->next = e; else p->head = e; p->tail = e;
}
static void port_enqueue(Entry* e, BOOL fromTimer) {
  Port* p = e->port; Queue* q = e->q;
  switch (p->mode) {
  case MODE_IMMEDIATE:
    if (fromTimer) { if (!QueueUserWorkItem(wi_run_entry, e, WT_EXECUTEDEFAULT)) run_entry(e, FALSE); }
    else run_entry(e, FALSE);
    return;
  case MODE_THREADPOOL:
    EnterCriticalSection(&p->cs); append(p, e); LeaveCriticalSection(&p->cs);
    if (!QueueUserWorkItem(wi_threadpool, p, WT_EXECUTEDEFAULT)) xgr_log("TQ: QueueUserWorkItem failed %lu", GetLastError());
    break;
  case MODE_SERIALIZED: {
    BOOL sched = FALSE;
    EnterCriticalSection(&p->cs); append(p, e); if (!p->busy) { p->busy = 1; sched = TRUE; } LeaveCriticalSection(&p->cs);
    if (sched && !QueueUserWorkItem(wi_serial, p, WT_EXECUTEDEFAULT)) xgr_log("TQ: QueueUserWorkItem failed %lu", GetLastError());
    break; }
  default: /* manual */
    EnterCriticalSection(&p->cs); append(p, e); SetEvent(p->avail); LeaveCriticalSection(&p->cs);
    break;
  }
  notify_monitors(q, e->portIdx);
  if (p->owner && p->owner != q) notify_monitors(p->owner, e->portIdx);
}

static DWORD WINAPI timer_thread(LPVOID unused) {
  for (;;) {
    Entry* due = NULL; Entry** tail = &due; DWORD wait = INFINITE;
    EnterCriticalSection(&g_tcs);
    ULONGLONG now = GetTickCount64();
    while (g_timers && g_timers->due <= now) { Entry* e = g_timers; g_timers = e->next; e->next = NULL; *tail = e; tail = &e->next; }
    if (g_timers) wait = (DWORD)(g_timers->due - now);
    LeaveCriticalSection(&g_tcs);
    while (due) { Entry* e = due; due = e->next; port_enqueue(e, TRUE); }
    WaitForSingleObject(g_tevent, wait);
  }
  return 0;
}
static void timer_add(Entry* e, UINT32 delay) {
  e->due = GetTickCount64() + delay;
  EnterCriticalSection(&g_tcs);
  Entry** pp = &g_timers; while (*pp && (*pp)->due <= e->due) pp = &(*pp)->next;
  e->next = *pp; *pp = e;
  LeaveCriticalSection(&g_tcs);
  SetEvent(g_tevent);
}

static HRESULT submit(void* qh, UINT32 portIdx, UINT32 delay, void* ctx, XTaskQueueCallback cb) {
  Queue* q = resolve(qh);
  if (!q || !cb || portIdx > 1) return E_INVALIDARG;
  Port* p = q->ports[portIdx];
  if (q_dead(q, p)) return E_ABORT;
  Entry* e = (Entry*)calloc(1, sizeof *e);
  e->cb = cb; e->ctx = ctx; e->q = q; e->port = p; e->portIdx = portIdx;
  InterlockedIncrement(&q->inflight);
  if (delay) timer_add(e, delay); else port_enqueue(e, FALSE);
  return S_OK;
}

static int own_frames(Queue* q) { int n = 0; for (Frame* f = T_FRAMES; f; f = f->prev) if (f->q == q) n++; return n; }
static void wait_inflight(Queue* q, int own) { while (q->inflight > own) Sleep(1); }

typedef struct { Queue* q; void* ctx; XTaskQueueTerminatedCallback cb; } TermCtx;
static DWORD WINAPI wi_term(LPVOID p) {
  TermCtx* t = (TermCtx*)p; wait_inflight(t->q, 0);
  if (t->cb) t->cb(t->ctx);
  free(t); return 0;
}
static BOOL cancel_match(Entry* e, Queue* q) { return e->q == q || e->port->owner == q; }
static HRESULT tq_terminate(void* qh, BOOL wait, void* ctx, XTaskQueueTerminatedCallback cb) {
  Queue* q = resolve(qh); if (!q) return E_INVALIDARG;
  xgr_log("TQ: terminate %p wait=%d", q, wait);
  InterlockedExchange(&q->terminated, 1);
  Entry* canc = NULL; Entry** tail = &canc;
  EnterCriticalSection(&g_tcs);
  for (Entry** pp = &g_timers; *pp;) {
    Entry* e = *pp; if (cancel_match(e, q)) { *pp = e->next; e->next = NULL; *tail = e; tail = &e->next; } else pp = &e->next;
  }
  LeaveCriticalSection(&g_tcs);
  for (int i = 0; i < 2; i++) {
    Port* p = q->ports[i]; if (i == 1 && p == q->ports[0]) break;
    EnterCriticalSection(&p->cs);
    Entry* last = NULL;
    for (Entry** pp = &p->head; *pp;) {
      Entry* e = *pp; if (cancel_match(e, q)) { *pp = e->next; e->next = NULL; *tail = e; tail = &e->next; } else { last = e; pp = &e->next; }
    }
    p->tail = last; if (!p->head) ResetEvent(p->avail);
    LeaveCriticalSection(&p->cs);
  }
  while (canc) { Entry* e = canc; canc = e->next; run_entry(e, TRUE); }
  if (wait) { wait_inflight(q, own_frames(q)); if (cb) cb(ctx); }
  else {
    TermCtx* t = (TermCtx*)calloc(1, sizeof *t); t->q = q; t->ctx = ctx; t->cb = cb;
    if (!QueueUserWorkItem(wi_term, t, WT_EXECUTEDEFAULT)) wi_term(t);
  }
  return S_OK;
}

// ---------------------------------------------------------------- XTaskQueue slots
#define A UINT64
#define IMPL(name) static UINT64 name(Obj* o, A a1, A a2, A a3, A a4, A a5, A a6)

IMPL(tq_create) {
  int wm = (int)(UINT32)a1, cm = (int)(UINT32)a2; void** out = (void**)a3;
  if (!out || wm < 0 || wm > 3 || cm < 0 || cm > 3) return E_INVALIDARG;
  ensure_init();
  Queue* q = queue_create(wm, cm); *out = q;
  xgr_log("TQ: XTaskQueueCreate(%d,%d) -> %p", wm, cm, q);
  return S_OK;
}
IMPL(tq_create_composite) {
  Port* w = (Port*)a1; Port* c = (Port*)a2; void** out = (void**)a3;
  if (!w || !c || !out || w->magic != PMAGIC || c->magic != PMAGIC) return E_INVALIDARG;
  ensure_init();
  Queue* q = queue_new(); q->ports[0] = w; q->ports[1] = c;
  if (w->owner) InterlockedIncrement(&w->owner->refs);
  if (c->owner) InterlockedIncrement(&c->owner->refs);
  *out = q; xgr_log("TQ: XTaskQueueCreateComposite(%p,%p) -> %p", w, c, q);
  return S_OK;
}
IMPL(tq_get_port) {
  Queue* q = resolve((void*)a1); UINT32 idx = (UINT32)a2; void** out = (void**)a3;
  if (!q || idx > 1 || !out) return E_INVALIDARG;
  *out = q->ports[idx]; return S_OK;
}
IMPL(tq_duplicate) {
  Queue* q = resolve((void*)a1); void** out = (void**)a2;
  if (!q || !out) return E_INVALIDARG;
  InterlockedIncrement(&q->refs); *out = q; return S_OK;
}
IMPL(tq_dispatch) {
  Queue* q = resolve((void*)a1); UINT32 idx = (UINT32)a2; UINT32 timeout = (UINT32)a3;
  if (!q || idx > 1) return 0;
  Port* p = q->ports[idx];
  ULONGLONG start = GetTickCount64();
  for (;;) {
    Entry* e = pop(p);
    if (e) { run_entry(e, FALSE); return 1; }
    if (timeout == 0 || p->mode != MODE_MANUAL) return 0;
    DWORD rem = INFINITE;
    if (timeout != INFINITE) {
      ULONGLONG el = GetTickCount64() - start; if (el >= timeout) return 0; rem = (DWORD)(timeout - el);
    }
    if (WaitForSingleObject(p->avail, rem) == WAIT_TIMEOUT) { Entry* e2 = pop(p); if (e2) { run_entry(e2, FALSE); return 1; } return 0; }
  }
}
IMPL(tq_close) {
  Queue* q = (Queue*)a1;
  if (q && q->magic == QMAGIC) InterlockedDecrement(&q->refs); // memory intentionally kept alive
  return 0;
}
IMPL(tq_submit) { return (UINT64)(LONG)submit((void*)a1, (UINT32)a2, 0, (void*)a3, (XTaskQueueCallback)a4); }
IMPL(tq_submit_delayed) { return (UINT64)(LONG)submit((void*)a1, (UINT32)a2, (UINT32)a3, (void*)a4, (XTaskQueueCallback)a5); }

typedef struct Waiter { Queue* q; UINT32 port; void* ctx; XTaskQueueCallback cb; HANDLE wh; } Waiter;
static VOID CALLBACK waiter_cb(PVOID p, BOOLEAN timedOut) { Waiter* w = (Waiter*)p; submit(w->q, w->port, 0, w->ctx, w->cb); }
IMPL(tq_register_waiter) {
  Queue* q = resolve((void*)a1); UINT32 port = (UINT32)a2; HANDLE h = (HANDLE)a3; UINT64* tok = (UINT64*)a6;
  if (!q || port > 1 || !h || !a5 || !tok) return E_INVALIDARG;
  Waiter* w = (Waiter*)calloc(1, sizeof *w); w->q = q; w->port = port; w->ctx = (void*)a4; w->cb = (XTaskQueueCallback)a5;
  if (!RegisterWaitForSingleObject(&w->wh, h, waiter_cb, w, INFINITE, WT_EXECUTEONLYONCE)) { free(w); return HRESULT_FROM_WIN32(GetLastError()); }
  *tok = (UINT64)(ULONG_PTR)w; return S_OK;
}
IMPL(tq_unregister_waiter) {
  Waiter* w = (Waiter*)(ULONG_PTR)a2; if (w) UnregisterWaitEx(w->wh, NULL); return 0; // w leaked (callback may race)
}
IMPL(tq_terminate_) { return (UINT64)(LONG)tq_terminate((void*)a1, (a2 & 0xff) != 0, (void*)a3, (XTaskQueueTerminatedCallback)a4); }
IMPL(tq_register_monitor) {
  Queue* q = resolve((void*)a1); XTaskQueueMonitorCallback cb = (XTaskQueueMonitorCallback)a3; UINT64* tok = (UINT64*)a4;
  if (!q || !cb || !tok) return E_INVALIDARG;
  Monitor* m = (Monitor*)calloc(1, sizeof *m); m->ctx = (void*)a2; m->cb = cb; m->token = (UINT64)InterlockedIncrement64(&g_token);
  EnterCriticalSection(&q->cs); m->next = q->monitors; q->monitors = m; LeaveCriticalSection(&q->cs);
  *tok = m->token;
  // libHttpClient: notify the new monitor about items already pending on manual ports
  for (UINT32 i = 0; i < 2; i++) { Port* p = q->ports[i]; if (p->mode == MODE_MANUAL && p->head) cb(m->ctx, q, i); }
  return S_OK;
}
IMPL(tq_unregister_monitor) {
  Queue* q = resolve((void*)a1); UINT64 tok = a2; if (!q) return 0;
  EnterCriticalSection(&q->cs);
  for (Monitor** pp = &q->monitors; *pp; pp = &(*pp)->next) if ((*pp)->token == tok) { *pp = (*pp)->next; break; } // node leaked (may be mid-call)
  LeaveCriticalSection(&q->cs);
  return 0;
}
IMPL(tq_get_process_queue) {
  void** out = (void**)a1; Queue* q = proc_queue();
  if (!q) { if (out) *out = NULL; return 0; }
  InterlockedIncrement(&q->refs); if (out) *out = q; return 1;
}
IMPL(tq_set_process_queue) {
  ensure_init(); Queue* q = (Queue*)a1;
  if (q && q->magic != QMAGIC) return 0;
  if (q) InterlockedIncrement(&q->refs);
  EnterCriticalSection(&g_acs); g_procQ = q; g_procSet = TRUE; LeaveCriticalSection(&g_acs);
  xgr_log("TQ: SetCurrentProcessTaskQueue(%p)", q);
  return 0;
}
IMPL(tq_nop) { return 0; }

// ---------------------------------------------------------------- XAsync
typedef struct XAsyncBlock XAsyncBlock;
typedef void (*XAsyncCompletionRoutine)(XAsyncBlock*);
struct XAsyncBlock { void* queue; void* context; XAsyncCompletionRoutine callback; unsigned char internal[sizeof(void*) * 4]; };
typedef struct { XAsyncBlock* async; size_t bufferSize; void* buffer; void* context; } XAsyncProviderData;
typedef HRESULT (*XAsyncProvider)(int op, const XAsyncProviderData* data);
typedef HRESULT (*XAsyncWork)(XAsyncBlock*);
enum { OP_BEGIN = 0, OP_DOWORK = 1, OP_GETRESULT = 2, OP_CANCEL = 3, OP_CLEANUP = 4 };

typedef struct AState {
  volatile LONG refs;
  XAsyncBlock* user;     // caller's block
  XAsyncBlock prov;      // copy handed to the provider (data.async)
  XAsyncProviderData pd; XAsyncProvider provider;
  const void* identity; const char* name;
  Queue* q; HANDLE done;
  volatile LONG completed, cleaned, canceled;
  HRESULT status; size_t needed;
} AState;
typedef struct { AState* state; HRESULT status; DWORD sig; } BlockInt;
#define ASIG 0x4E595341UL
static BlockInt* bi(XAsyncBlock* b) { return (BlockInt*)b->internal; }

static void a_release(AState* s) {
  if (InterlockedDecrement(&s->refs) == 0) { CloseHandle(s->done); free(s); }
}
// Returns state with an added ref (or NULL); *status gets the block's status.
static AState* a_get(XAsyncBlock* b, HRESULT* status) {
  ensure_init(); AState* s = NULL; HRESULT st = E_INVALIDARG;
  if (!b) { if (status) *status = st; return NULL; }
  EnterCriticalSection(&g_acs);
  BlockInt* x = bi(b);
  if (x->sig == ASIG) { st = x->status; s = x->state; if (s) InterlockedIncrement(&s->refs); }
  LeaveCriticalSection(&g_acs);
  if (status) *status = st;
  return s;
}
static void a_unlink_locked(AState* s) {
  BlockInt* u = bi(s->user); if (u->sig == ASIG && u->state == s) { u->state = NULL; u->status = s->status; }
  BlockInt* p = bi(&s->prov); if (p->state == s) { p->state = NULL; p->status = s->status; }
}
static BOOL a_cleanup_provider(AState* s) {
  if (InterlockedExchange(&s->cleaned, 1)) return FALSE;
  s->provider(OP_CLEANUP, &s->pd); return TRUE;
}
static void a_completion_cb(void* ctx, unsigned char canceled) {
  AState* s = (AState*)ctx;
  if (s->prov.callback) s->prov.callback(s->user);   // user block must not be touched after this
  SetEvent(s->done); a_release(s);                    // releases the "alive" ref
}
static void a_complete(XAsyncBlock* b, HRESULT hr, size_t needed) {
  if (hr == E_PENDING) return;
  AState* s = a_get(b, NULL); if (!s) return;
  if (InterlockedCompareExchange(&s->completed, 1, 0) != 0) { a_release(s); return; }
  s->status = hr; s->needed = SUCCEEDED(hr) ? needed : 0;
  BOOL cleanup = FAILED(hr) || s->needed == 0;
  if (cleanup) a_cleanup_provider(s);
  EnterCriticalSection(&g_acs);
  bi(s->user)->status = hr; bi(&s->prov)->status = hr;   // publish status atomically w/ unlink
  if (cleanup) a_unlink_locked(s);
  LeaveCriticalSection(&g_acs);
  if (cleanup) a_release(s);  // link ref
  if (FAILED(hr)) xgr_log("XAsync: %s completed hr=%08lx", s->name ? s->name : "?", (unsigned long)hr);
  if (s->prov.callback) {
    if (FAILED(submit(s->q, 1, 0, s, a_completion_cb))) a_completion_cb(s, 1);
  } else { SetEvent(s->done); a_release(s); }
  a_release(s);  // temp ref
}
static void a_dowork_cb(void* ctx, unsigned char canceled) {
  AState* s = (AState*)ctx;
  if (!s->completed) {
    if (canceled || s->canceled) a_complete(&s->prov, E_ABORT, 0);
    else {
      HRESULT hr = s->provider(OP_DOWORK, &s->pd);
      if (hr != E_PENDING) a_complete(&s->prov, hr, 0);
    }
  }
  a_release(s);
}
static HRESULT a_schedule(XAsyncBlock* b, UINT32 delay) {
  AState* s = a_get(b, NULL); if (!s) return E_INVALIDARG;
  if (s->completed) { a_release(s); return E_ILLEGAL_METHOD_CALL_; }
  HRESULT hr = submit(s->q, 0, delay, s, a_dowork_cb);   // ref transferred to entry
  if (FAILED(hr)) a_release(s);
  return hr;
}
static HRESULT a_begin(XAsyncBlock* b, void* ctx, const void* identity, const char* name, XAsyncProvider provider) {
  ensure_init();
  if (!b || !provider) return E_INVALIDARG;
  Queue* q = resolve(b->queue); if (!q) return E_INVALIDARG;
  AState* s = (AState*)calloc(1, sizeof *s);
  s->refs = 2; /* link + alive */ s->user = b; s->prov = *b; s->provider = provider; s->identity = identity; s->name = name;
  s->q = q; InterlockedIncrement(&q->refs);
  s->done = CreateEventW(NULL, TRUE, FALSE, NULL); s->status = E_PENDING;
  s->pd.async = &s->prov; s->pd.context = ctx;
  EnterCriticalSection(&g_acs);
  BlockInt* u = bi(b);
  if (u->sig == ASIG && u->state && !u->state->completed) {
    LeaveCriticalSection(&g_acs); CloseHandle(s->done); free(s);
    xgr_log("XAsyncBegin: block %p already in use (%s)", b, name ? name : "?"); return E_INVALIDARG;
  }
  memset(b->internal, 0, sizeof b->internal); memset(s->prov.internal, 0, sizeof s->prov.internal);
  *bi(b) = (BlockInt){ s, E_PENDING, ASIG }; *bi(&s->prov) = (BlockInt){ s, E_PENDING, ASIG };
  LeaveCriticalSection(&g_acs);
  HRESULT hr = provider(OP_BEGIN, &s->pd);
  if (FAILED(hr) && hr != E_PENDING) {
    xgr_log("XAsyncBegin(%s): provider Begin failed %08lx", name ? name : "?", (unsigned long)hr);
    if (InterlockedCompareExchange(&s->completed, 1, 0) == 0) {
      s->status = hr; a_cleanup_provider(s);
      EnterCriticalSection(&g_acs); a_unlink_locked(s); LeaveCriticalSection(&g_acs);
      SetEvent(s->done); a_release(s); a_release(s);
    }
    return hr;
  }
  return S_OK;
}

IMPL(xa_get_status) {
  XAsyncBlock* b = (XAsyncBlock*)a1; BOOL wait = (a2 & 0xff) != 0; HRESULT st;
  AState* s = a_get(b, &st);
  if (s) {
    if (wait && st == E_PENDING) { WaitForSingleObject(s->done, INFINITE); st = s->status; }
    a_release(s);
  }
  return (UINT64)(LONG)st;
}
IMPL(xa_get_result_size) {
  XAsyncBlock* b = (XAsyncBlock*)a1; size_t* sz = (size_t*)a2; HRESULT st;
  AState* s = a_get(b, &st);
  if (!sz) { if (s) a_release(s); return E_INVALIDARG; }
  *sz = 0;
  if (st == E_PENDING) st = E_ILLEGAL_METHOD_CALL_;
  else if (SUCCEEDED(st) && s) *sz = s->needed;
  if (s) a_release(s);
  return (UINT64)(LONG)st;
}
IMPL(xa_cancel) {
  AState* s = a_get((XAsyncBlock*)a1, NULL); if (!s) return 0;
  if (!s->completed) {
    InterlockedExchange(&s->canceled, 1);
    HRESULT hr = s->provider(OP_CANCEL, &s->pd);
    if (FAILED(hr)) a_complete(&s->prov, E_ABORT, 0);
  }
  a_release(s); return 0;
}
static HRESULT run_provider(int op, const XAsyncProviderData* d) {
  switch (op) {
  case OP_BEGIN: return a_schedule(d->async, 0);
  case OP_DOWORK: return ((XAsyncWork)d->context)(d->async);
  default: return S_OK;
  }
}
IMPL(xa_run) {
  if (!a2) return E_INVALIDARG;
  return (UINT64)(LONG)a_begin((XAsyncBlock*)a1, (void*)a2, (const void*)run_provider, "XAsyncRun", run_provider);
}
IMPL(xa_begin) {
  return (UINT64)(LONG)a_begin((XAsyncBlock*)a1, (void*)a2, (const void*)a3, (const char*)a4, (XAsyncProvider)a5);
}
IMPL(xa_schedule) { return (UINT64)(LONG)a_schedule((XAsyncBlock*)a1, (UINT32)a2); }
IMPL(xa_complete) { a_complete((XAsyncBlock*)a1, (HRESULT)(UINT32)a2, (size_t)a3); return 0; }
IMPL(xa_get_result) {
  XAsyncBlock* b = (XAsyncBlock*)a1; const void* identity = (const void*)a2; size_t size = (size_t)a3; void* buf = (void*)a4; size_t* used = (size_t*)a5;
  HRESULT st; AState* s = a_get(b, &st);
  if (used) *used = 0;
  if (st == E_PENDING) { if (s) a_release(s); return E_ILLEGAL_METHOD_CALL_; }
  if (FAILED(st) || !s) { if (s) a_release(s); return (UINT64)(LONG)st; }
  if (identity != s->identity) xgr_log("XAsyncGetResult: identity mismatch (%p vs %p, %s)", identity, s->identity, s->name ? s->name : "?");
  HRESULT hr = st;
  if (s->needed) {
    if (size < s->needed || !buf) { a_release(s); return E_NOT_SUFFICIENT_BUFFER_; }
    XAsyncProviderData d = s->pd; d.buffer = buf; d.bufferSize = s->needed;
    hr = s->provider(OP_GETRESULT, &d);
    if (used) *used = s->needed;
  }
  a_cleanup_provider(s);
  EnterCriticalSection(&g_acs);
  BOOL linked = bi(s->user)->state == s || bi(&s->prov)->state == s;
  a_unlink_locked(s);
  LeaveCriticalSection(&g_acs);
  if (linked) a_release(s);  // link ref
  a_release(s);              // temp ref
  return (UINT64)(LONG)hr;
}

const Impl g_taskqueue_impls[] = {
  {TQ_PROV, 3, xa_get_status, "XAsyncGetStatus"},
  {TQ_PROV, 4, xa_get_result_size, "XAsyncGetResultSize"},
  {TQ_PROV, 5, xa_cancel, "XAsyncCancel"},
  {TQ_PROV, 6, xa_run, "XAsyncRun"},
  {TQ_PROV, 7, xa_begin, "XAsyncBegin"},
  {TQ_PROV, 9, xa_schedule, "XAsyncSchedule"},
  {TQ_PROV, 10, xa_complete, "XAsyncComplete"},
  {TQ_PROV, 11, xa_get_result, "XAsyncGetResult"},
  {TQ_PROV, 12, tq_create, "XTaskQueueCreate"},
  {TQ_PROV, 13, tq_create_composite, "XTaskQueueCreateComposite"},
  {TQ_PROV, 14, tq_get_port, "XTaskQueueGetPort"},
  {TQ_PROV, 15, tq_duplicate, "XTaskQueueDuplicateHandle"},
  {TQ_PROV, 16, tq_dispatch, "XTaskQueueDispatch"},
  {TQ_PROV, 17, tq_close, "XTaskQueueCloseHandle"},
  {TQ_PROV, 18, tq_submit, "XTaskQueueSubmitCallback"},
  {TQ_PROV, 19, tq_submit_delayed, "XTaskQueueSubmitDelayedCallback"},
  {TQ_PROV, 20, tq_register_waiter, "XTaskQueueRegisterWaiter"},
  {TQ_PROV, 21, tq_unregister_waiter, "XTaskQueueUnregisterWaiter"},
  {TQ_PROV, 22, tq_terminate_, "XTaskQueueTerminate"},
  {TQ_PROV, 23, tq_register_monitor, "XTaskQueueRegisterMonitor"},
  {TQ_PROV, 24, tq_unregister_monitor, "XTaskQueueUnregisterMonitor"},
  {TQ_PROV, 25, tq_get_process_queue, "XTaskQueueGetCurrentProcessTaskQueue"},
  {TQ_PROV, 26, tq_set_process_queue, "XTaskQueueSetCurrentProcessTaskQueue"},
  {TQ_PROV, 29, tq_nop, "slot29-nop"},
  {0, 0, 0, 0}
};
