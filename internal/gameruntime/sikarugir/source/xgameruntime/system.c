#include "xgr.h"
#include <stdlib.h>
// Non-TaskQueue GDK families. Slot map: /Users/user/Bedrock/diagnostics/gdk_thunks.md
// Methods are COM-style: this in rcx, then args. Struct-returning methods get the
// hidden return pointer as a1 and must return it. Bool methods must return 0/1.

typedef struct { UINT16 major, minor, build, revision; } XVersion;
typedef struct { XVersion osVersion, hostingOsVersion; char family[64]; char form[64]; } XSystemAnalyticsInfo;

#define IMPL(name) static UINT64 name(Obj* o, UINT64 a1, UINT64 a2, UINT64 a3, UINT64 a4, UINT64 a5, UINT64 a6)

// b884675d s3: XSystemAnalyticsInfo XSystemGetAnalyticsInfo()
IMPL(XSystemGetAnalyticsInfo_) {
  XSystemAnalyticsInfo* r = (XSystemAnalyticsInfo*)a1; memset(r, 0, sizeof *r);
  r->osVersion = (XVersion){10, 0, 19045, 0}; r->hostingOsVersion = r->osVersion;
  strcpy(r->family, "Windows.Desktop"); strcpy(r->form, "Desktop");
  return a1;
}
IMPL(ret_false) { return 0; }
IMPL(ret_true) { return 1; }
IMPL(ret_void) { return 0; }
IMPL(ret_s_ok) { return S_OK; }
IMPL(ret_int0) { return 0; }

// Registration with token out-param at a given arg position: always succeed, hand out a token.
static volatile LONG64 g_token = 1;
IMPL(reg_token_a4) { if (a4) *(UINT64*)a4 = InterlockedIncrement64(&g_token); return S_OK; }
IMPL(reg_token_a5) { if (a5) *(UINT64*)a5 = InterlockedIncrement64(&g_token); return S_OK; }

// XGameRuntimeIsFeatureAvailable(XGameRuntimeFeature): env XGR_FEATURES = hex bitmask of features
// reported available (default: all).
IMPL(XGameRuntimeIsFeatureAvailable_) {
  static UINT64 mask = 0; static int init;
  if (!init) { char b[32]; mask = GetEnvironmentVariableA("XGR_FEATURES", b, sizeof b) ? strtoull(b, 0, 16) : ~0ull; init = 1; }
  return (a1 < 64 && (mask >> a1) & 1) ? 1 : 0;
}

// E_GAMEUSER_NO_DEFAULT_USER-ish: no signed-in Xbox user under Wine.
#define E_GAMEUSER_NO_DEFAULT_USER ((HRESULT)0x8015DC12)
// Experimental (see /Users/user/Bedrock/diagnostics/signin.md), all OFF unless env is set:
//   XGR_XUSER_ADD    = "sync:<hex>" | "async:<hex>"  behaviour for XUserAddAsync(AddDefaultUserSilently)
//   XGR_XUSER_ADD_UI = same, for XUserAddAsync(options without the silent bit, i.e. "show UI")
//   sync  : return <hex> synchronously (legacy behaviour; XUserAddResult then also returns <hex>)
//   async : start the XAsync op and complete it with <hex> on the block's queue (like the real
//           runtime does when there is no user); XUserAddResult returns <hex>, *newUser = NULL.
// With XGR_XUSER_ADD unset: sync 0x8015DC12, and XUserAddResult -> 0x8015DC12 / NULL handle.
typedef struct { void* queue; void* context; void* callback; unsigned char internal[32]; } XUAsyncBlock;
typedef struct { XUAsyncBlock* async; size_t bufferSize; void* buffer; void* context; } XUProviderData;
static impl_fn tq_fn(int slot) {
  for (int k = 0; g_taskqueue_impls[k].fn; k++) if (g_taskqueue_impls[k].slot == slot) return g_taskqueue_impls[k].fn;
  return 0;
}
static HRESULT xuadd_provider(int op, const XUProviderData* d) {
  if (op == 0) return (HRESULT)tq_fn(9)(0, (UINT64)d->async, 0, 0, 0, 0, 0);   // Begin -> XAsyncSchedule
  if (op == 1) return (HRESULT)(LONG)(INT_PTR)d->context;                       // DoWork -> fail hr
  return S_OK;
}
static int xuadd_mode(int ui, HRESULT* hr) {   // 0 = legacy, 1 = sync, 2 = async
  char b[64]; const char* v = NULL;
  if (ui && GetEnvironmentVariableA("XGR_XUSER_ADD_UI", b, sizeof b)) v = b;
  else if (GetEnvironmentVariableA("XGR_XUSER_ADD", b, sizeof b)) v = b;
  if (!v) { *hr = E_GAMEUSER_NO_DEFAULT_USER; return 0; }
  const char* c = strchr(v, ':'); *hr = c ? (HRESULT)strtoul(c + 1, 0, 16) : E_GAMEUSER_NO_DEFAULT_USER;
  return strncmp(v, "async", 5) == 0 ? 2 : 1;
}
static struct { void* block; HRESULT hr; } g_xuadd[16]; static LONG g_xuadd_i;
IMPL(XUserAddAsync_) {
  HRESULT hr; int mode = xuadd_mode(!(a1 & 1), &hr);
  if (mode) xgr_log("XUserAddAsync(options=%llx, block=%llx) mode=%s hr=%08lx", a1, a2, mode == 2 ? "async" : "sync", (unsigned long)hr);
  if (mode == 2 && a2) {
    HRESULT b = (HRESULT)tq_fn(7)(0, a2, (UINT64)(INT_PTR)hr, (UINT64)xuadd_provider, (UINT64)"XUserAddAsync", (UINT64)xuadd_provider, 0);
    if (SUCCEEDED(b)) return S_OK;
    xgr_log("XUserAddAsync: XAsyncBegin failed %08lx, falling back to sync", (unsigned long)b);
  }
  if (mode && a2) { LONG i = InterlockedIncrement(&g_xuadd_i) & 15; g_xuadd[i].block = (void*)a2; g_xuadd[i].hr = hr; }
  return (UINT64)(LONG)hr;
}
// XUserAddResult(XAsyncBlock* async, XUserHandle* newUser): report the failure, never a handle.
// Default (env unset): 0x8015DC12. With XGR_XUSER_ADD set: the hr the op was completed with.
IMPL(XUserAddResult_) {
  if (a2) *(void**)a2 = NULL;
  HRESULT hr; if (!xuadd_mode(0, &hr)) return (UINT64)E_GAMEUSER_NO_DEFAULT_USER;
  HRESULT st = (HRESULT)tq_fn(3)(0, a1, 0, 0, 0, 0, 0);                       // XAsyncGetStatus(no wait)
  if (st != E_INVALIDARG) {                                                    // our async op
    if (st == E_PENDING) return (UINT64)(LONG)0x8000000EL;                     // E_ILLEGAL_METHOD_CALL
    st = (HRESULT)tq_fn(11)(0, a1, (UINT64)xuadd_provider, 0, 0, 0, 0);        // XAsyncGetResult -> hr, cleans up
  } else {                                                                     // sync-failed block
    st = hr;
    for (int i = 0; i < 16; i++) if (g_xuadd[i].block == (void*)a1) st = g_xuadd[i].hr;
  }
  xgr_log("XUserAddResult(block=%llx) -> %08lx", a1, (unsigned long)st);
  return (UINT64)(LONG)(SUCCEEDED(st) ? E_GAMEUSER_NO_DEFAULT_USER : st);
}

// XPackageGetCurrentProcessPackageIdentifier(size_t bufferSize, char* buffer)
IMPL(XPackageGetCurrentProcessPackageIdentifier_) {
  const char* id = "Microsoft.MinecraftUWP_8wekyb3d8bbwe";
  if (!a2 || a1 <= strlen(id)) return (UINT64)HRESULT_FROM_WIN32(ERROR_INSUFFICIENT_BUFFER);
  strcpy((char*)a2, id); return S_OK;
}

// e349bd1a slot 4 (both iids dadc2895/67ce4bfc): XSystemGetXboxLiveSandboxId(size_t size, char* buf, size_t* used).
// The Xbox Live services library needs this; retail titles run in the RETAIL sandbox.
IMPL(XSystemGetXboxLiveSandboxId_) {
  const char* sb = "RETAIL"; size_t n = strlen(sb) + 1;
  if (a3) *(size_t*)a3 = n;
  if (!a2 || a1 < n) return (UINT64)HRESULT_FROM_WIN32(ERROR_INSUFFICIENT_BUFFER);
  memcpy((char*)a2, sb, n); xgr_log("XSystemGetXboxLiveSandboxId -> RETAIL"); return S_OK;
}
// 973a344e/2549f142 slot 3: called once by the Xbox Live services library during initialization with one
// pointer argument; exact API unknown. Succeed so XSAPI initializes.
IMPL(Xbl973_slot3_) { xgr_log("973a344e slot3(%p) -> S_OK", (void*)a1); return S_OK; }

const Impl g_system_impls[] = {
  {0xe349bd1a, 4, XSystemGetXboxLiveSandboxId_, "XSystemGetXboxLiveSandboxId"},
  {0x973a344e, 3, Xbl973_slot3_, "XSAPI runtime init (973a344e slot 3)"},
  {0xb884675d, 3, XSystemGetAnalyticsInfo_, "XSystemGetAnalyticsInfo"},
  {0x8836fe87, 3, XGameRuntimeIsFeatureAvailable_, "XGameRuntimeIsFeatureAvailable"},
  {0x8ca467f7, 4, ret_void, "XErrorSetCallback"},
  // XUser (01acd177)
  {0x01acd177, 5, ret_int0, "XUserCompare"},
  {0x01acd177, 7, XUserAddAsync_, "XUserAddAsync"},
  {0x01acd177, 8, XUserAddResult_, "XUserAddResult"},
  {0x01acd177, 13, ret_false, "XUserGetIsGuest"},
  {0x01acd177, 33, reg_token_a4, "XUserRegisterForChangeEvent"},
  {0x01acd177, 34, ret_true, "XUserUnregisterForChangeEvent"},
  // XGameInvite (95fd18d2), XGameProtocol (0651aae2)
  {0x95fd18d2, 3, reg_token_a4, "XGameInviteRegisterForEvent"},
  {0x95fd18d2, 4, ret_true, "XGameInviteUnregisterForEvent"},
  {0x0651aae2, 3, reg_token_a4, "XGameProtocolRegisterForActivation"},
  {0x0651aae2, 4, ret_true, "XGameProtocolUnregisterForActivation"},
  // XNetworking (37e56907)
  {0x37e56907, 16, reg_token_a4, "XNetworkingRegisterConnectivityHintChanged"},
  {0x37e56907, 17, ret_true, "XNetworkingUnregisterConnectivityHintChanged"},
  // XPackage (af406016)
  {0xaf406016, 11, XPackageGetCurrentProcessPackageIdentifier_, "XPackageGetCurrentProcessPackageIdentifier"},
  // XStore (0dd112ac)
  {0x0dd112ac, 66, reg_token_a5, "XStoreRegisterGameLicenseChanged"},
  {0x0dd112ac, 67, ret_true, "XStoreUnregisterGameLicenseChanged"},
  {0, 0, 0, 0}
};
