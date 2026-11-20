#include "xgr.h"
#include <stdlib.h>
// XNetworking (provider 37e56907, iids bf2346b2-... used by the game/XCurl and 37e56907-... used by
// libHttpClient; both share the same slot layout). Slot map: /Users/user/Bedrock/diagnostics/gdk_thunks.md,
// details: /Users/user/Bedrock/diagnostics/networking.md.
//
//  s8  XNetworkingQuerySecurityInformationForUrlAsync(const char* url, XAsyncBlock*)
//  s9  ...AsyncResultSize(XAsyncBlock*, size_t*)
//  s10 ...AsyncResult(XAsyncBlock*, size_t bufSize, size_t* used, uint8_t* buf, XNetworkingSecurityInformation**)
//  s11..s13  same, Utf16 URL (XCurl uses these before every request)
//  s14 XNetworkingVerifyServerCertificate(HINTERNET request, const XNetworkingSecurityInformation*)
//  s15 XNetworkingGetConnectivityHint(XNetworkingConnectivityHint*)
//  s16 XNetworkingRegisterConnectivityHintChanged(queue, ctx, cb, token*)
//  s17 XNetworkingUnregisterConnectivityHintChanged(token, bool wait) -> bool
//
// libHttpClient holds every HTTP call until a connectivity-changed callback reports
// networkInitialized=true (it re-queries s15 from inside the callback), so the callback
// must fire at least once.
//
// Env: XGR_NET_OFFLINE=1 reports no connectivity (for testing the offline paths).

#define NET_PROV 0x37e56907
#define TQ_PROV 0x073b7dcb
#define IMPL(name) static UINT64 name(Obj* o, UINT64 a1, UINT64 a2, UINT64 a3, UINT64 a4, UINT64 a5, UINT64 a6)

typedef struct {
  UINT32 connectivityLevel;   // 0 Unknown, 1 None, 2 LocalAccess, 3 InternetAccess, 4 ConstrainedInternetAccess
  UINT32 connectivityCost;    // 0 Unknown, 1 Unrestricted, 2 Fixed, 3 Variable
  UINT32 ianaInterfaceType;   // 6 = ethernetCsmacd, 71 = ieee80211
  BOOLEAN networkInitialized, approachingDataLimit, overDataLimit, roaming;
} XNetworkingConnectivityHint;  // 16 bytes; offset 0xC networkInitialized (verified in libHttpClient.GDK.dll)
typedef void (*HintChangedCb)(void* ctx, const XNetworkingConnectivityHint* hint);

typedef struct { UINT32 thumbprintType; size_t size; UINT8* buffer; } XNetworkingThumbprintData;
typedef struct {
  UINT32 enabledHttpSecurityProtocolFlags;  // WINHTTP_FLAG_SECURE_PROTOCOL_* (XCurl passes it to WinHttpSetOption 84)
  size_t thumbprintCount;
  const XNetworkingThumbprintData* thumbprints;
} XNetworkingSecurityInformation;

// ---- task-queue / XAsync helpers (resolved from taskqueue.c's table by name)
static impl_fn tq(const char* name) {
  for (int k = 0; g_taskqueue_impls[k].fn; k++)
    if (g_taskqueue_impls[k].prov1 == TQ_PROV && !strcmp(g_taskqueue_impls[k].name, name)) return g_taskqueue_impls[k].fn;
  xgr_log("NET: taskqueue impl %s missing", name); return 0;
}
static UINT64 tq_call(const char* name, const UINT64* a) {
  impl_fn f = tq(name); return f ? f(0, a[0], a[1], a[2], a[3], a[4], a[5]) : (UINT64)E_NOTIMPL;
}
#define TQCALL(name, ...) tq_call(name, (const UINT64[6]){__VA_ARGS__})

static void fill_hint(XNetworkingConnectivityHint* h) {
  static int offline = -1;
  if (offline < 0) { char b[8]; offline = GetEnvironmentVariableA("XGR_NET_OFFLINE", b, sizeof b) && b[0] == '1'; }
  memset(h, 0, sizeof *h);
  h->connectivityLevel = offline ? 1 : 3;
  h->connectivityCost = 1;
  h->ianaInterfaceType = 6;
  h->networkInitialized = 1;
}

IMPL(GetConnectivityHint_) {
  if (!a1) return E_INVALIDARG;
  fill_hint((XNetworkingConnectivityHint*)a1);
  return S_OK;
}

// ---- connectivity-changed registrations
typedef struct Reg { struct Reg* next; UINT64 token; void* queue; void* ctx; HintChangedCb cb; volatile LONG live; } Reg;
static Reg* g_regs; static SRWLOCK g_regs_lock = SRWLOCK_INIT;
static volatile LONG64 g_net_token = 0x4e000000;

static void fire_cb(void* p, unsigned char canceled) {
  Reg* r = (Reg*)p;
  if (canceled || !r->live) return;
  XNetworkingConnectivityHint h; fill_hint(&h);
  xgr_log("NET: connectivity callback cb=%p ctx=%p (level=%u init=%u)", r->cb, r->ctx, h.connectivityLevel, h.networkInitialized);
  r->cb(r->ctx, &h);
}

IMPL(RegisterConnectivityHintChanged_) {
  void* queue = (void*)a1; void* ctx = (void*)a2; HintChangedCb cb = (HintChangedCb)a3; UINT64* token = (UINT64*)a4;
  if (!cb || !token) return E_INVALIDARG;
  Reg* r = (Reg*)calloc(1, sizeof *r);
  r->token = InterlockedIncrement64(&g_net_token); r->queue = queue; r->ctx = ctx; r->cb = cb; r->live = 1;
  AcquireSRWLockExclusive(&g_regs_lock); r->next = g_regs; g_regs = r; ReleaseSRWLockExclusive(&g_regs_lock);
  *token = r->token;
  // Fire once shortly after registration (on the queue's completion port), like the real runtime
  // does when the network state becomes known.
  HRESULT hr = (HRESULT)TQCALL("XTaskQueueSubmitDelayedCallback", (UINT64)queue, 1 /*Completion*/, 100, (UINT64)r, (UINT64)fire_cb);
  xgr_log("NET: RegisterConnectivityHintChanged queue=%p cb=%p token=%llx -> submit %08lx", queue, cb, r->token, (unsigned long)hr);
  return S_OK;
}

IMPL(UnregisterConnectivityHintChanged_) {
  AcquireSRWLockShared(&g_regs_lock);
  for (Reg* r = g_regs; r; r = r->next) if (r->token == a1) InterlockedExchange(&r->live, 0);
  ReleaseSRWLockShared(&g_regs_lock);
  return 1;  // bool; Reg nodes are leaked on purpose (a queued fire_cb may still reference them)
}

// ---- security information for URL (XCurl queries this before each request)
static const XNetworkingSecurityInformation* sec_template(void) {
  static XNetworkingSecurityInformation s;
  static int init;
  if (!init) {
    char b[32];
    // TLS1.2 | TLS1.3 by default; XGR_TLS_FLAGS (hex) overrides.
    s.enabledHttpSecurityProtocolFlags = GetEnvironmentVariableA("XGR_TLS_FLAGS", b, sizeof b) ? strtoul(b, 0, 16) : 0x00000800 | 0x00002000;
    init = 1;
  }
  return &s;
}
typedef struct { void* async; size_t bufferSize; void* buffer; void* context; } ProvData;
enum { OP_BEGIN = 0, OP_DOWORK = 1, OP_GETRESULT = 2, OP_CANCEL = 3, OP_CLEANUP = 4 };
static HRESULT sec_provider(int op, const ProvData* d) {
  switch (op) {
  case OP_BEGIN: return (HRESULT)TQCALL("XAsyncSchedule", (UINT64)d->async, 0);
  case OP_DOWORK: TQCALL("XAsyncComplete", (UINT64)d->async, S_OK, sizeof(XNetworkingSecurityInformation)); return S_OK;
  case OP_GETRESULT:
    if (d->bufferSize < sizeof(XNetworkingSecurityInformation)) return E_NOT_SUFFICIENT_BUFFER;
    memcpy(d->buffer, sec_template(), sizeof(XNetworkingSecurityInformation));
    return S_OK;
  default: return S_OK;
  }
}
IMPL(QuerySecInfoUtf8Async_) {
  xgr_log("NET: QuerySecurityInformationForUrl(\"%s\")", a1 ? (const char*)a1 : "(null)");
  return TQCALL("XAsyncBegin", a2, 0, (UINT64)sec_provider, (UINT64)"XNetworkingQuerySecurityInformationForUrlAsync", (UINT64)sec_provider);
}
IMPL(QuerySecInfoUtf16Async_) {
  xgr_log("NET: QuerySecurityInformationForUrlUtf16(\"%ls\")", a1 ? (const wchar_t*)a1 : L"(null)");
  return TQCALL("XAsyncBegin", a2, 0, (UINT64)sec_provider, (UINT64)"XNetworkingQuerySecurityInformationForUrlUtf16Async", (UINT64)sec_provider);
}
IMPL(QuerySecInfoResultSize_) { return TQCALL("XAsyncGetResultSize", a1, a2); }
IMPL(QuerySecInfoResult_) {
  UINT64 hr = TQCALL("XAsyncGetResult", a1, (UINT64)sec_provider, a2, a4, a3);
  if (a5) *(void**)a5 = SUCCEEDED((HRESULT)hr) ? (void*)a4 : NULL;
  if (FAILED((HRESULT)hr)) xgr_log("NET: QuerySecurityInformation result hr=%08lx", (unsigned long)hr);
  return hr;
}
IMPL(VerifyServerCertificate_) { return S_OK; }  // no pinning: WinHTTP/schannel already validated the chain

const Impl g_networking_impls[] = {
  {NET_PROV, 8, QuerySecInfoUtf8Async_, "XNetworkingQuerySecurityInformationForUrlAsync"},
  {NET_PROV, 9, QuerySecInfoResultSize_, "XNetworkingQuerySecurityInformationForUrlAsyncResultSize"},
  {NET_PROV, 10, QuerySecInfoResult_, "XNetworkingQuerySecurityInformationForUrlAsyncResult"},
  {NET_PROV, 11, QuerySecInfoUtf16Async_, "XNetworkingQuerySecurityInformationForUrlUtf16Async"},
  {NET_PROV, 12, QuerySecInfoResultSize_, "XNetworkingQuerySecurityInformationForUrlUtf16AsyncResultSize"},
  {NET_PROV, 13, QuerySecInfoResult_, "XNetworkingQuerySecurityInformationForUrlUtf16AsyncResult"},
  {NET_PROV, 14, VerifyServerCertificate_, "XNetworkingVerifyServerCertificate"},
  {NET_PROV, 15, GetConnectivityHint_, "XNetworkingGetConnectivityHint"},
  {NET_PROV, 16, RegisterConnectivityHintChanged_, "XNetworkingRegisterConnectivityHintChanged"},
  {NET_PROV, 17, UnregisterConnectivityHintChanged_, "XNetworkingUnregisterConnectivityHintChanged"},
  {0, 0, 0, 0}
};
