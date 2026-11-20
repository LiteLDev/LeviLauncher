// mcwinrt.dll - minimal WinRT activatable-class shim for Minecraft Bedrock (GDK) under Wine.
// Implements just enough of:
//   Windows.ApplicationModel.DataTransfer.DataTransferManager (statics + interop + instance)
//   Windows.UI.Text.Core.CoreTextServicesManager (statics, manager, CoreTextEditContext)
//   Windows.Storage.StorageFile (statics -> fail)
//   Windows.Storage.Pickers.FileSavePicker (instance; PickSaveFileAsync -> completed/null = cancelled)
//   Microsoft.Windows.Storage.Pickers.FileOpenPicker (WinAppSDK; PickSingleFileAsync -> null)
// Every call and every unknown QI is logged to C:\mcwinrt.log and stderr (prefix "[mcwinrt]").
//
// Object model: an Obj owns up to MAXT "tear-offs"; each tear-off is {vtbl, owner} and is the
// interface pointer handed out. All tear-offs share the owner's refcount.
#include <windows.h>
#include <stdio.h>
#include <stdarg.h>
#include <string.h>

typedef struct HSTRING__ *HSTRING;
typedef struct Obj Obj;
typedef struct Tear { void **vtbl; Obj *o; } Tear;
#define MAXT 6
struct Obj {
    LONG ref;
    const WCHAR *rc;          // runtime class name
    const char *tag;          // for logs
    int n;
    Tear t[MAXT];
    const GUID *iid[MAXT];    // iid[i] identifies t[i]
    void *handler;            // async: completion handler
    HWND hwnd;
};

/* ---------------- logging ---------------- */
static CRITICAL_SECTION g_cs;
static HANDLE g_logf = INVALID_HANDLE_VALUE;
static void logf_(const char *fmt, ...) {
    char buf[1024];
    int n = snprintf(buf, sizeof buf, "[mcwinrt %04lx] ", GetCurrentThreadId());
    va_list ap; va_start(ap, fmt);
    n += vsnprintf(buf + n, sizeof buf - n - 2, fmt, ap);
    va_end(ap);
    if (n > (int)sizeof buf - 2) n = sizeof buf - 2;
    buf[n++] = '\n'; buf[n] = 0;
    EnterCriticalSection(&g_cs);
    fputs(buf, stderr); fflush(stderr);
    if (g_logf == INVALID_HANDLE_VALUE)
        g_logf = CreateFileA("C:\\mcwinrt.log", FILE_APPEND_DATA, FILE_SHARE_READ | FILE_SHARE_WRITE, NULL,
                             OPEN_ALWAYS, FILE_ATTRIBUTE_NORMAL, NULL);
    if (g_logf != INVALID_HANDLE_VALUE) { DWORD w; WriteFile(g_logf, buf, n, &w, NULL); }
    LeaveCriticalSection(&g_cs);
}
#define LOG(...) logf_(__VA_ARGS__)
static const char *gstr(const GUID *g, char *b) {
    sprintf(b, "{%08lx-%04x-%04x-%02x%02x-%02x%02x%02x%02x%02x%02x}", g->Data1, g->Data2, g->Data3,
            g->Data4[0], g->Data4[1], g->Data4[2], g->Data4[3], g->Data4[4], g->Data4[5], g->Data4[6], g->Data4[7]);
    return b;
}

/* ---------------- combase string helpers (resolved at runtime) ---------------- */
typedef HRESULT (WINAPI *pWindowsCreateString)(const WCHAR *, UINT32, HSTRING *);
typedef const WCHAR *(WINAPI *pWindowsGetStringRawBuffer)(HSTRING, UINT32 *);
typedef HRESULT (WINAPI *pRoGetActivationFactory)(HSTRING, REFIID, void **);
typedef HRESULT (WINAPI *pWindowsDeleteString)(HSTRING);
static pWindowsCreateString fCreate; static pWindowsGetStringRawBuffer fRaw;
static pRoGetActivationFactory fRoGAF; static pWindowsDeleteString fDelete;
static void init_combase(void) {
    if (fCreate) return;
    HMODULE h = LoadLibraryA("combase.dll");
    fCreate = (pWindowsCreateString)GetProcAddress(h, "WindowsCreateString");
    fRaw = (pWindowsGetStringRawBuffer)GetProcAddress(h, "WindowsGetStringRawBuffer");
    fRoGAF = (pRoGetActivationFactory)GetProcAddress(h, "RoGetActivationFactory");
    fDelete = (pWindowsDeleteString)GetProcAddress(h, "WindowsDeleteString");
}
static const WCHAR *hraw(HSTRING s) { UINT32 l; const WCHAR *p = s && fRaw ? fRaw(s, &l) : NULL; return p ? p : L""; }

/* ---------------- GUIDs ---------------- */
#define G(name, l, w1, w2, b0, b1, b2, b3, b4, b5, b6, b7) \
    static const GUID name = {l, w1, w2, {b0, b1, b2, b3, b4, b5, b6, b7}}
G(IID_IUnk,        0x00000000, 0x0000, 0x0000, 0xc0,0,0,0,0,0,0,0x46);
G(IID_IInsp,       0xaf86e2e0, 0xb12d, 0x4c6a, 0x9c,0x5a,0xd7,0xaa,0x65,0x10,0x1e,0x90);
G(IID_IAgile,      0x94ea2b94, 0xe9cc, 0x49e0, 0xc0,0xff,0xee,0x64,0xca,0x8f,0x5b,0x90);
G(IID_IActFactory, 0x00000035, 0x0000, 0x0000, 0xc0,0,0,0,0,0,0,0x46);
G(IID_IAsyncInfo,  0x00000036, 0x0000, 0x0000, 0xc0,0,0,0,0,0,0,0x46);
// DataTransfer
G(IID_IDTMInterop, 0x3a3dcd6c, 0x3eab, 0x43dc, 0xbc,0xde,0x45,0x67,0x1c,0xe8,0x00,0xc8);
G(IID_IDTM,        0xa5caee9b, 0x8708, 0x49d1, 0x8d,0x36,0x67,0xd2,0x5a,0x8d,0xa0,0x0c);
G(IID_IDTM2,       0x30ae7d71, 0x8ba8, 0x4c02, 0x8e,0x3f,0xdd,0xb2,0x3b,0x38,0x87,0x15);
G(IID_IDTMStatics, 0xa9da01aa, 0xe00e, 0x4cfe, 0xaa,0x44,0x2d,0xd9,0x32,0xdc,0xa3,0xd8);
G(IID_IDTMStatics2,0xc54ec2ec, 0x9f97, 0x4d63, 0x98,0x68,0x39,0x5e,0x27,0x1a,0xd8,0xf5);
G(IID_IDTMStatics3,0x05845473, 0x6c82, 0x4f5c, 0xac,0x23,0x62,0xe4,0x58,0x36,0x1f,0xac);
// Text.Core
G(IID_ICTSMStatics,0x1520a388, 0xe2cf, 0x4d65, 0xae,0xb9,0xb3,0x2d,0x86,0xfe,0x39,0xb9);
G(IID_ICTSM,       0xc2507d83, 0x6e0a, 0x4a8a, 0xbd,0xf8,0x19,0x48,0x87,0x48,0x54,0xba);
G(IID_ICTEC,       0xbf6608af, 0x4041, 0x47c3, 0xb2,0x63,0xa9,0x18,0xeb,0x5e,0xae,0xf2);
G(IID_ICTEC2,      0xb1867dbb, 0x083b, 0x49e1, 0xb2,0x81,0x2b,0x35,0xd6,0x2b,0xf4,0x66);
G(IID_ILangFactory,0x9b0252ac, 0x0c27, 0x44f8, 0xb7,0x92,0x97,0x93,0xfb,0x66,0xc6,0x3e);
// Storage
G(IID_IStorageFileStatics, 0x5984c710, 0xdaf2, 0x43c8, 0x8b,0xb4,0xa4,0xd3,0xea,0xcf,0xd0,0x3f);
G(IID_IFileSavePicker,     0x3286ffcb, 0x617f, 0x4cc5, 0xaf,0x6a,0xb3,0xfd,0xf2,0x9a,0xd1,0x45);
G(IID_IInitWithWindow,     0x3e68d4bd, 0x7135, 0x4d10, 0x80,0x18,0x9f,0xb6,0xd9,0xf3,0x3f,0xa1);
G(IID_ISdkOpenPickerFactory,0x315e86d7,0xd7a2, 0x5d81, 0xb3,0x79,0x7a,0xf7,0x82,0x07,0xb1,0xaf);
// Placeholders for parameterised/unknown default interfaces (never match a real QI)
G(IID_NONE,        0xffffffff, 0xffff, 0xffff, 0xff,0xff,0xff,0xff,0xff,0xff,0xff,0xff);

/* ---------------- IUnknown / IInspectable ---------------- */
static ULONG WINAPI U_AddRef(Tear *s) { return InterlockedIncrement(&s->o->ref); }
static ULONG WINAPI U_Release(Tear *s) {
    Obj *o = s->o; LONG r = InterlockedDecrement(&o->ref);
    if (r == 0) {
        if (o->handler) ((IUnknown *)o->handler)->lpVtbl->Release((IUnknown *)o->handler);
        HeapFree(GetProcessHeap(), 0, o);
    }
    return r;
}
static HRESULT WINAPI U_QI(Tear *s, REFIID riid, void **out) {
    Obj *o = s->o; char b[64];
    if (!out) return E_POINTER;
    *out = NULL;
    if (IsEqualGUID(riid, &IID_IUnk) || IsEqualGUID(riid, &IID_IInsp) || IsEqualGUID(riid, &IID_IAgile)) {
        *out = &o->t[0];
    } else {
        for (int i = 0; i < o->n; i++)
            if (IsEqualGUID(riid, o->iid[i])) { *out = &o->t[i]; break; }
    }
    if (!*out) { LOG("%s: QI unknown %s -> E_NOINTERFACE", o->tag, gstr(riid, b)); return E_NOINTERFACE; }
    InterlockedIncrement(&o->ref);
    return S_OK;
}
static HRESULT WINAPI I_GetIids(Tear *s, ULONG *cnt, GUID **iids) {
    Obj *o = s->o; int n = 0;
    GUID *a = CoTaskMemAlloc(sizeof(GUID) * MAXT);
    for (int i = 0; i < o->n; i++) if (o->iid[i] != &IID_NONE) a[n++] = *o->iid[i];
    *cnt = n; *iids = a; LOG("%s: GetIids -> %d", o->tag, n); return S_OK;
}
static HRESULT WINAPI I_GetRCN(Tear *s, HSTRING *out) {
    init_combase(); LOG("%s: GetRuntimeClassName", s->o->tag);
    return fCreate(s->o->rc, lstrlenW(s->o->rc), out);
}
static HRESULT WINAPI I_GetTrust(Tear *s, int *lvl) { *lvl = 0; return S_OK; }

#define IUNK   (void *)U_QI, (void *)U_AddRef, (void *)U_Release
#define IINSP  IUNK, (void *)I_GetIids, (void *)I_GetRCN, (void *)I_GetTrust

static Obj *obj_new(const WCHAR *rc, const char *tag) {
    Obj *o = HeapAlloc(GetProcessHeap(), HEAP_ZERO_MEMORY, sizeof(Obj));
    o->ref = 1; o->rc = rc; o->tag = tag;
    return o;
}
static void obj_add(Obj *o, const GUID *iid, void **vtbl) {
    o->t[o->n].vtbl = vtbl; o->t[o->n].o = o; o->iid[o->n] = iid; o->n++;
}
#define ITF(o, i) ((void *)&(o)->t[i])

/* ---------------- stub-method generators ---------------- */
static volatile LONG64 g_tok = 0x1000;
#define TAGOF(s) ((s)->o->tag)
#define M_EVENT(name) \
    static HRESULT WINAPI name##_add(Tear *s, void *h, INT64 *tok) { \
        INT64 t = InterlockedIncrement64(&g_tok); if (tok) *tok = t; \
        LOG("%s: add_" #name "(%p) -> token %lld", TAGOF(s), h, t); return S_OK; } \
    static HRESULT WINAPI name##_rm(Tear *s, INT64 tok) { LOG("%s: remove_" #name "(%lld)", TAGOF(s), tok); return S_OK; }
#define EVENT(name) (void *)name##_add, (void *)name##_rm
#define M_PUT(name) static HRESULT WINAPI name(Tear *s, UINT64 v) { LOG("%s: " #name "(0x%llx)", TAGOF(s), v); return S_OK; }
#define M_GETP(name) static HRESULT WINAPI name(Tear *s, void **v) { LOG("%s: " #name " -> null", TAGOF(s)); if (v) *v = NULL; return S_OK; }
#define M_GETI(name, val) static HRESULT WINAPI name(Tear *s, INT32 *v) { LOG("%s: " #name " -> %d", TAGOF(s), (int)(val)); if (v) *v = (val); return S_OK; }
#define M_GETB(name, val) static HRESULT WINAPI name(Tear *s, BYTE *v) { LOG("%s: " #name " -> %d", TAGOF(s), (int)(val)); if (v) *v = (val); return S_OK; }
#define M_NOP(name) static HRESULT WINAPI name(Tear *s) { LOG("%s: " #name "()", TAGOF(s)); return S_OK; }
#define M_FAIL(name, hr) static HRESULT WINAPI name(Tear *s) { LOG("%s: " #name " -> 0x%08lx", TAGOF(s), (unsigned long)(hr)); return (hr); }
// Generic getter returning an empty HSTRING (NULL == "")
#define M_GETS(name) M_GETP(name)

/* ================================================================
 * Async operation that is already completed with a null result
 * (IAsyncOperation<T> for T = runtime class; used for pickers = "user cancelled")
 * ================================================================ */
typedef struct { void **vtbl; } Delegate; // IUnknown-based; Invoke at slot 3
static HRESULT WINAPI AO_put_Completed(Tear *s, IUnknown *h) {
    Obj *o = s->o;
    LOG("%s: put_Completed(%p) -> invoking with Completed", o->tag, h);
    if (!h) return S_OK;
    h->lpVtbl->AddRef(h);
    o->handler = h;
    typedef HRESULT (WINAPI *inv_t)(void *, void *, int);
    inv_t inv = (inv_t)((Delegate *)h)->vtbl[3];
    HRESULT hr = inv(h, &o->t[0], 1 /* AsyncStatus::Completed */);
    LOG("%s: handler returned 0x%08lx", o->tag, hr);
    return S_OK;
}
static HRESULT WINAPI AO_get_Completed(Tear *s, IUnknown **h) {
    LOG("%s: get_Completed", TAGOF(s));
    *h = s->o->handler; if (*h) (*h)->lpVtbl->AddRef(*h); return S_OK;
}
static HRESULT WINAPI AO_GetResults(Tear *s, void **r) { LOG("%s: GetResults -> null", TAGOF(s)); *r = NULL; return S_OK; }
static void *vt_AsyncOp[] = { IINSP, AO_put_Completed, AO_get_Completed, AO_GetResults };
static HRESULT WINAPI AI_get_Id(Tear *s, UINT32 *id) { *id = 1; LOG("%s: get_Id", TAGOF(s)); return S_OK; }
M_GETI(AI_get_Status, 1)
static HRESULT WINAPI AI_get_ErrorCode(Tear *s, HRESULT *e) { *e = S_OK; LOG("%s: get_ErrorCode", TAGOF(s)); return S_OK; }
M_NOP(AI_Cancel) M_NOP(AI_Close)
static void *vt_AsyncInfo[] = { IINSP, AI_get_Id, AI_get_Status, AI_get_ErrorCode, AI_Cancel, AI_Close };
static void *new_null_async(const char *tag) {
    Obj *o = obj_new(L"Windows.Foundation.IAsyncOperation`1", tag);
    obj_add(o, &IID_NONE, vt_AsyncOp);
    obj_add(o, &IID_IAsyncInfo, vt_AsyncInfo);
    return ITF(o, 0);
}

/* ================================================================
 * Minimal collections: IMap<K,V> and IVector<T> that accept writes and stay empty
 * ================================================================ */
static HRESULT WINAPI Map_Lookup(Tear *s, void *k, void **v) { LOG("%s: Lookup", TAGOF(s)); *v = NULL; return E_BOUNDS; }
M_GETI(Map_get_Size, 0)
M_GETB(Map_HasKey_b, 0)
static HRESULT WINAPI Map_HasKey(Tear *s, void *k, BYTE *v) { LOG("%s: HasKey -> false", TAGOF(s)); *v = 0; return S_OK; }
static HRESULT WINAPI Map_GetView(Tear *s, void **v) { LOG("%s: GetView -> E_NOTIMPL", TAGOF(s)); *v = NULL; return E_NOTIMPL; }
static HRESULT WINAPI Map_Insert(Tear *s, void *k, void *v, BYTE *rep) {
    LOG("%s: Insert(%S)", TAGOF(s), hraw((HSTRING)k)); if (rep) *rep = 0; return S_OK; }
static HRESULT WINAPI Map_Remove(Tear *s, void *k) { LOG("%s: Remove", TAGOF(s)); return S_OK; }
M_NOP(Map_Clear)
static void *vt_Map[] = { IINSP, Map_Lookup, Map_get_Size, Map_HasKey, Map_GetView, Map_Insert, Map_Remove, Map_Clear };

static HRESULT WINAPI Vec_GetAt(Tear *s, UINT32 i, void **v) { LOG("%s: GetAt(%u) -> E_BOUNDS", TAGOF(s), i); *v = NULL; return E_BOUNDS; }
M_GETI(Vec_get_Size, 0)
static HRESULT WINAPI Vec_GetView(Tear *s, void **v) { LOG("%s: GetView -> E_NOTIMPL", TAGOF(s)); *v = NULL; return E_NOTIMPL; }
static HRESULT WINAPI Vec_IndexOf(Tear *s, void *x, UINT32 *i, BYTE *f) { LOG("%s: IndexOf", TAGOF(s)); *i = 0; *f = 0; return S_OK; }
static HRESULT WINAPI Vec_SetAt(Tear *s, UINT32 i, void *x) { LOG("%s: SetAt", TAGOF(s)); return S_OK; }
static HRESULT WINAPI Vec_InsertAt(Tear *s, UINT32 i, void *x) { LOG("%s: InsertAt", TAGOF(s)); return S_OK; }
static HRESULT WINAPI Vec_RemoveAt(Tear *s, UINT32 i) { LOG("%s: RemoveAt", TAGOF(s)); return S_OK; }
static HRESULT WINAPI Vec_Append(Tear *s, void *x) { LOG("%s: Append", TAGOF(s)); return S_OK; }
M_NOP(Vec_RemoveAtEnd) M_NOP(Vec_Clear)
static HRESULT WINAPI Vec_GetMany(Tear *s, UINT32 st, UINT32 cap, void *items, UINT32 *got) { LOG("%s: GetMany", TAGOF(s)); *got = 0; return S_OK; }
static HRESULT WINAPI Vec_ReplaceAll(Tear *s, UINT32 n, void *items) { LOG("%s: ReplaceAll(%u items)", TAGOF(s), n); return S_OK; }
static void *vt_Vec[] = { IINSP, Vec_GetAt, Vec_get_Size, Vec_GetView, Vec_IndexOf, Vec_SetAt, Vec_InsertAt,
                          Vec_RemoveAt, Vec_Append, Vec_RemoveAtEnd, Vec_Clear, Vec_GetMany, Vec_ReplaceAll };

/* ================================================================
 * IActivationFactory (shared)
 * ================================================================ */
static HRESULT WINAPI AF_ActivateInstance_notimpl(Tear *s, void **out) {
    LOG("%s: ActivateInstance -> E_NOTIMPL", TAGOF(s)); if (out) *out = NULL; return E_NOTIMPL; }
static void *vt_AF_static[] = { IINSP, AF_ActivateInstance_notimpl };

/* ================================================================
 * DataTransferManager
 * ================================================================ */
M_EVENT(DataRequested) M_EVENT(TargetApplicationChosen) M_EVENT(ShareProvidersRequested)
static void *vt_DTM[] = { IINSP, EVENT(DataRequested), EVENT(TargetApplicationChosen) };
static void *vt_DTM2[] = { IINSP, EVENT(ShareProvidersRequested) };
static Obj *new_dtm(HWND hwnd) {
    Obj *o = obj_new(L"Windows.ApplicationModel.DataTransfer.DataTransferManager", "DataTransferManager");
    o->hwnd = hwnd;
    obj_add(o, &IID_IDTM, vt_DTM);
    obj_add(o, &IID_IDTM2, vt_DTM2);
    return o;
}
static HRESULT WINAPI DTMI_GetForWindow(Tear *s, HWND hwnd, REFIID riid, void **out) {
    char b[64];
    Obj *o = new_dtm(hwnd);
    HRESULT hr = U_QI(&o->t[0], riid, out);
    U_Release(&o->t[0]);
    LOG("DTMInterop: GetForWindow(%p, %s) -> 0x%08lx %p", hwnd, gstr(riid, b), hr, out ? *out : NULL);
    return hr;
}
static HRESULT WINAPI DTMI_ShowShareUIForWindow(Tear *s, HWND hwnd) {
    LOG("DTMInterop: ShowShareUIForWindow(%p) -> E_NOTIMPL", hwnd); return E_NOTIMPL; }
static void *vt_DTMInterop[] = { IUNK, DTMI_GetForWindow, DTMI_ShowShareUIForWindow };
static HRESULT WINAPI DTMS_GetForCurrentView(Tear *s, void **out) {
    Obj *o = new_dtm(NULL); *out = ITF(o, 0); LOG("DTMStatics: GetForCurrentView -> %p", *out); return S_OK; }
M_NOP(DTMS_ShowShareUI)
static void *vt_DTMStatics[] = { IINSP, DTMS_GetForCurrentView, DTMS_ShowShareUI };
M_GETB(DTMS2_IsSupported, 0)
static void *vt_DTMStatics2[] = { IINSP, DTMS2_IsSupported };
M_FAIL(DTMS3_ShowShareUIWithOptions, E_NOTIMPL)
static void *vt_DTMStatics3[] = { IINSP, DTMS3_ShowShareUIWithOptions };
static Obj *factory_dtm(void) {
    Obj *o = obj_new(L"Windows.ApplicationModel.DataTransfer.DataTransferManager", "DTMFactory");
    obj_add(o, &IID_IActFactory, vt_AF_static);
    obj_add(o, &IID_IDTMInterop, vt_DTMInterop);
    obj_add(o, &IID_IDTMStatics, vt_DTMStatics);
    obj_add(o, &IID_IDTMStatics2, vt_DTMStatics2);
    obj_add(o, &IID_IDTMStatics3, vt_DTMStatics3);
    return o;
}

/* ================================================================
 * CoreTextServicesManager / CoreTextEditContext
 * ================================================================ */
M_GETS(CTEC_get_Name) M_PUT(CTEC_put_Name)
M_GETI(CTEC_get_InputScope, 0) M_PUT(CTEC_put_InputScope)
M_GETB(CTEC_get_IsReadOnly, 0) M_PUT(CTEC_put_IsReadOnly)
M_GETI(CTEC_get_InputPaneDisplayPolicy, 0) M_PUT(CTEC_put_InputPaneDisplayPolicy)
M_EVENT(TextRequested) M_EVENT(SelectionRequested) M_EVENT(LayoutRequested) M_EVENT(TextUpdating)
M_EVENT(SelectionUpdating) M_EVENT(FormatUpdating) M_EVENT(CompositionStarted) M_EVENT(CompositionCompleted)
M_EVENT(FocusRemoved) M_EVENT(NotifyFocusLeaveCompleted)
M_NOP(CTEC_NotifyFocusEnter) M_NOP(CTEC_NotifyFocusLeave)
static HRESULT WINAPI CTEC_NotifyTextChanged(Tear *s, UINT64 range, INT32 len, UINT64 sel) {
    LOG("%s: NotifyTextChanged(len=%d)", TAGOF(s), len); return S_OK; }
static HRESULT WINAPI CTEC_NotifySelectionChanged(Tear *s, UINT64 sel) {
    LOG("%s: NotifySelectionChanged([%d,%d])", TAGOF(s), (int)(sel & 0xffffffff), (int)(sel >> 32)); return S_OK; }
M_NOP(CTEC_NotifyLayoutChanged)
static void *vt_CTEC[] = { IINSP,
    CTEC_get_Name, CTEC_put_Name, CTEC_get_InputScope, CTEC_put_InputScope,
    CTEC_get_IsReadOnly, CTEC_put_IsReadOnly, CTEC_get_InputPaneDisplayPolicy, CTEC_put_InputPaneDisplayPolicy,
    EVENT(TextRequested), EVENT(SelectionRequested), EVENT(LayoutRequested), EVENT(TextUpdating),
    EVENT(SelectionUpdating), EVENT(FormatUpdating), EVENT(CompositionStarted), EVENT(CompositionCompleted),
    EVENT(FocusRemoved), CTEC_NotifyFocusEnter, CTEC_NotifyFocusLeave, CTEC_NotifyTextChanged,
    CTEC_NotifySelectionChanged, CTEC_NotifyLayoutChanged };
static void *vt_CTEC2[] = { IINSP, EVENT(NotifyFocusLeaveCompleted) };

static HRESULT WINAPI CTSM_get_InputLanguage(Tear *s, void **out) {
    // Return a real Windows.Globalization.Language("en-US") from Wine if possible.
    HSTRING cls = NULL, tag = NULL; void *fac = NULL; HRESULT hr;
    *out = NULL;
    init_combase();
    fCreate(L"Windows.Globalization.Language", 30, &cls);
    hr = fRoGAF(cls, &IID_ILangFactory, &fac);
    if (SUCCEEDED(hr) && fac) {
        fCreate(L"en-US", 5, &tag);
        typedef HRESULT (WINAPI *cl_t)(void *, HSTRING, void **);
        hr = ((cl_t)(*(void ***)fac)[6])(fac, tag, out);
        ((IUnknown *)fac)->lpVtbl->Release((IUnknown *)fac);
        fDelete(tag);
    }
    fDelete(cls);
    LOG("CTSM: get_InputLanguage -> 0x%08lx %p", hr, *out);
    if (FAILED(hr)) { *out = NULL; return S_OK; }
    return hr;
}
M_EVENT(InputLanguageChanged)
static HRESULT WINAPI CTSM_CreateEditContext(Tear *s, void **out) {
    Obj *o = obj_new(L"Windows.UI.Text.Core.CoreTextEditContext", "CoreTextEditContext");
    obj_add(o, &IID_ICTEC, vt_CTEC);
    obj_add(o, &IID_ICTEC2, vt_CTEC2);
    *out = ITF(o, 0);
    LOG("CTSM: CreateEditContext -> %p", *out);
    return S_OK;
}
static void *vt_CTSM[] = { IINSP, CTSM_get_InputLanguage, EVENT(InputLanguageChanged), CTSM_CreateEditContext };
static HRESULT WINAPI CTSMS_GetForCurrentView(Tear *s, void **out) {
    static Obj *g_mgr;
    if (!g_mgr) {
        g_mgr = obj_new(L"Windows.UI.Text.Core.CoreTextServicesManager", "CoreTextServicesManager");
        obj_add(g_mgr, &IID_ICTSM, vt_CTSM);
    }
    InterlockedIncrement(&g_mgr->ref);
    *out = ITF(g_mgr, 0);
    LOG("CTSMStatics: GetForCurrentView -> %p", *out);
    return S_OK;
}
static void *vt_CTSMStatics[] = { IINSP, CTSMS_GetForCurrentView };
static Obj *factory_ctsm(void) {
    Obj *o = obj_new(L"Windows.UI.Text.Core.CoreTextServicesManager", "CTSMFactory");
    obj_add(o, &IID_IActFactory, vt_AF_static);
    obj_add(o, &IID_ICTSMStatics, vt_CTSMStatics);
    return o;
}

/* ================================================================
 * StorageFile statics: all fail with ERROR_FILE_NOT_FOUND
 * ================================================================ */
#define HR_FNF ((HRESULT)0x80070002)
M_FAIL(SFS_GetFileFromPathAsync, HR_FNF) M_FAIL(SFS_GetFileFromApplicationUriAsync, HR_FNF)
M_FAIL(SFS_CreateStreamedFileAsync, E_NOTIMPL) M_FAIL(SFS_ReplaceWithStreamedFileAsync, E_NOTIMPL)
M_FAIL(SFS_CreateStreamedFileFromUriAsync, E_NOTIMPL) M_FAIL(SFS_ReplaceWithStreamedFileFromUriAsync, E_NOTIMPL)
static void *vt_SFStatics[] = { IINSP, SFS_GetFileFromPathAsync, SFS_GetFileFromApplicationUriAsync,
    SFS_CreateStreamedFileAsync, SFS_ReplaceWithStreamedFileAsync, SFS_CreateStreamedFileFromUriAsync,
    SFS_ReplaceWithStreamedFileFromUriAsync };
static Obj *factory_storagefile(void) {
    Obj *o = obj_new(L"Windows.Storage.StorageFile", "StorageFileFactory");
    obj_add(o, &IID_IActFactory, vt_AF_static);
    obj_add(o, &IID_IStorageFileStatics, vt_SFStatics);
    return o;
}

/* ================================================================
 * Windows.Storage.Pickers.FileSavePicker
 * ================================================================ */
M_GETS(FSP_get_SettingsIdentifier) M_PUT(FSP_put_SettingsIdentifier)
M_GETI(FSP_get_SuggestedStartLocation, 0) M_PUT(FSP_put_SuggestedStartLocation)
M_GETS(FSP_get_CommitButtonText) M_PUT(FSP_put_CommitButtonText)
static HRESULT WINAPI FSP_get_FileTypeChoices(Tear *s, void **out) {
    Obj *o = obj_new(L"Windows.Foundation.Collections.IMap`2", "FileTypeChoicesMap");
    obj_add(o, &IID_NONE, vt_Map);
    *out = ITF(o, 0); LOG("FileSavePicker: get_FileTypeChoices -> %p", *out); return S_OK;
}
M_GETS(FSP_get_DefaultFileExtension) M_PUT(FSP_put_DefaultFileExtension)
M_GETP(FSP_get_SuggestedSaveFile) M_PUT(FSP_put_SuggestedSaveFile)
M_GETS(FSP_get_SuggestedFileName)
static HRESULT WINAPI FSP_put_SuggestedFileName(Tear *s, HSTRING v) { LOG("FileSavePicker: put_SuggestedFileName(%S)", hraw(v)); return S_OK; }
static HRESULT WINAPI FSP_PickSaveFileAsync(Tear *s, void **out) {
    *out = new_null_async("PickSaveFileAsync");
    LOG("FileSavePicker: PickSaveFileAsync -> completed(null) %p", *out); return S_OK;
}
static void *vt_FSP[] = { IINSP, FSP_get_SettingsIdentifier, FSP_put_SettingsIdentifier,
    FSP_get_SuggestedStartLocation, FSP_put_SuggestedStartLocation, FSP_get_CommitButtonText,
    FSP_put_CommitButtonText, FSP_get_FileTypeChoices, FSP_get_DefaultFileExtension,
    FSP_put_DefaultFileExtension, FSP_get_SuggestedSaveFile, FSP_put_SuggestedSaveFile,
    FSP_get_SuggestedFileName, FSP_put_SuggestedFileName, FSP_PickSaveFileAsync };
static HRESULT WINAPI IWW_Initialize(Tear *s, HWND h) { LOG("%s: IInitializeWithWindow::Initialize(%p)", TAGOF(s), h); s->o->hwnd = h; return S_OK; }
static void *vt_IWW[] = { IUNK, IWW_Initialize };
static HRESULT WINAPI FSPF_ActivateInstance(Tear *s, void **out) {
    Obj *o = obj_new(L"Windows.Storage.Pickers.FileSavePicker", "FileSavePicker");
    obj_add(o, &IID_IFileSavePicker, vt_FSP);
    obj_add(o, &IID_IInitWithWindow, vt_IWW);
    *out = ITF(o, 0); LOG("FileSavePicker: ActivateInstance -> %p", *out); return S_OK;
}
static void *vt_AF_FSP[] = { IINSP, FSPF_ActivateInstance };
static Obj *factory_fsp(void) {
    Obj *o = obj_new(L"Windows.Storage.Pickers.FileSavePicker", "FileSavePickerFactory");
    obj_add(o, &IID_IActFactory, vt_AF_FSP);
    return o;
}

/* ================================================================
 * Microsoft.Windows.Storage.Pickers.FileOpenPicker (WinAppSDK 1.8)
 * slot 12 = get_FileTypeFilter (IVector<String>), slot 13 = PickSingleFileAsync
 * ================================================================ */
M_GETI(SOP_get6, 0) M_PUT(SOP_put7) M_GETI(SOP_get8, 0) M_PUT(SOP_put9) M_GETS(SOP_get10) M_PUT(SOP_put11)
static HRESULT WINAPI SOP_get_FileTypeFilter(Tear *s, void **out) {
    Obj *o = obj_new(L"Windows.Foundation.Collections.IVector`1", "FileTypeFilterVector");
    obj_add(o, &IID_NONE, vt_Vec);
    *out = ITF(o, 0); LOG("SdkFileOpenPicker: get_FileTypeFilter -> %p", *out); return S_OK;
}
static HRESULT WINAPI SOP_PickSingleFileAsync(Tear *s, void **out) {
    *out = new_null_async("PickSingleFileAsync");
    LOG("SdkFileOpenPicker: PickSingleFileAsync -> completed(null) %p", *out); return S_OK;
}
static HRESULT WINAPI SOP_PickMultipleFilesAsync(Tear *s, void **out) {
    LOG("SdkFileOpenPicker: PickMultipleFilesAsync -> E_NOTIMPL"); *out = NULL; return E_NOTIMPL; }
M_NOP(SOP_s15) M_NOP(SOP_s16) M_NOP(SOP_s17) M_NOP(SOP_s18) M_NOP(SOP_s19) M_NOP(SOP_s20)
static void *vt_SOP[] = { IINSP, SOP_get6, SOP_put7, SOP_get8, SOP_put9, SOP_get10, SOP_put11,
    SOP_get_FileTypeFilter, SOP_PickSingleFileAsync, SOP_PickMultipleFilesAsync,
    SOP_s15, SOP_s16, SOP_s17, SOP_s18, SOP_s19, SOP_s20 };
static HRESULT WINAPI SOPF_CreateInstance(Tear *s, UINT64 windowId, void **out) {
    Obj *o = obj_new(L"Microsoft.Windows.Storage.Pickers.FileOpenPicker", "SdkFileOpenPicker");
    obj_add(o, &IID_NONE, vt_SOP);
    *out = ITF(o, 0); LOG("SdkFileOpenPickerFactory: CreateInstance(windowId=0x%llx) -> %p", windowId, *out); return S_OK;
}
static void *vt_SOPFactory[] = { IINSP, SOPF_CreateInstance };
static Obj *factory_sop(void) {
    Obj *o = obj_new(L"Microsoft.Windows.Storage.Pickers.FileOpenPicker", "SdkFileOpenPickerFactory");
    obj_add(o, &IID_IActFactory, vt_AF_static);
    obj_add(o, &IID_ISdkOpenPickerFactory, vt_SOPFactory);
    return o;
}

/* ================================================================
 * Windows.UI.ViewManagement.Core.CoreInputView (overrides Wine's windows.ui.dll version, whose
 * TryHidePrimaryView returns E_NOTIMPL and makes the game throw hresult_not_implemented)
 * ================================================================ */
G(IID_ICIVStatics,  0x7d9b97cd, 0xedbe, 0x49cf, 0xa5,0x4f,0x33,0x7d,0xe0,0x52,0x90,0x7f);
G(IID_ICIVStatics2, 0x7ebc0862, 0xd049, 0x4e52, 0x87,0xb0,0x1e,0x90,0xe9,0x8c,0x49,0xed);
G(IID_ICIV,  0xc770cd7a, 0x7001, 0x4c32, 0xbf,0x94,0x25,0xc1,0xf5,0x54,0xcb,0xf1);
G(IID_ICIV2, 0x0ed726c1, 0xe09a, 0x4ae8, 0xae,0xdf,0xdf,0xa4,0x85,0x7d,0x1a,0x01);
G(IID_ICIV3, 0xbc941653, 0x3ab9, 0x4849, 0x8f,0x58,0x46,0xe7,0xf0,0x35,0x3c,0xfc);
G(IID_ICIV4, 0x002863d6, 0xd9ef, 0x57eb, 0x8c,0xef,0x77,0xf6,0xce,0x1b,0x7e,0xe7);
static HRESULT WINAPI VV_GetAt(Tear *s, UINT32 i, void **v) { LOG("%s: GetAt(%u) -> E_BOUNDS", TAGOF(s), i); return E_BOUNDS; }
M_GETI(VV_get_Size, 0)
static HRESULT WINAPI VV_IndexOf(Tear *s, void *x, UINT32 *i, BYTE *f) { LOG("%s: IndexOf", TAGOF(s)); *i = 0; *f = 0; return S_OK; }
static HRESULT WINAPI VV_GetMany(Tear *s, UINT32 st, UINT32 cap, void *items, UINT32 *got) { LOG("%s: GetMany", TAGOF(s)); *got = 0; return S_OK; }
static void *vt_VecView[] = { IINSP, VV_GetAt, VV_get_Size, VV_IndexOf, VV_GetMany };
M_EVENT(OcclusionsChanged) M_EVENT(XYFocusTransferringFromPrimaryView) M_EVENT(XYFocusTransferredToPrimaryView)
M_EVENT(PrimaryViewShowing) M_EVENT(PrimaryViewHiding)
static HRESULT WINAPI CIV_GetCoreInputViewOcclusions(Tear *s, void **out) {
    Obj *o = obj_new(L"Windows.Foundation.Collections.IVectorView`1", "OcclusionsView");
    obj_add(o, &IID_NONE, vt_VecView);
    *out = ITF(o, 0); LOG("CoreInputView: GetCoreInputViewOcclusions -> empty %p", *out); return S_OK;
}
M_GETB(CIV_TryShowPrimaryView, 0) M_GETB(CIV_TryHidePrimaryView, 0)
static void *vt_CIV[] = { IINSP, EVENT(OcclusionsChanged), CIV_GetCoreInputViewOcclusions, CIV_TryShowPrimaryView, CIV_TryHidePrimaryView };
static HRESULT WINAPI CIV2_TryTransferXYFocus(Tear *s, void *rect, INT32 dir, BYTE *r) { LOG("CoreInputView: TryTransferXYFocusToPrimaryView -> false"); *r = 0; return S_OK; }
static void *vt_CIV2[] = { IINSP, EVENT(XYFocusTransferringFromPrimaryView), EVENT(XYFocusTransferredToPrimaryView), CIV2_TryTransferXYFocus };
M_GETB(CIV3_TryShow, 0)
static HRESULT WINAPI CIV3_TryShowWithKind(Tear *s, INT32 kind, BYTE *r) { LOG("CoreInputView: TryShowWithKind(%d) -> false", kind); *r = 0; return S_OK; }
M_GETB(CIV3_TryHide, 0)
static void *vt_CIV3[] = { IINSP, CIV3_TryShow, CIV3_TryShowWithKind, CIV3_TryHide };
static void *vt_CIV4[] = { IINSP, EVENT(PrimaryViewShowing), EVENT(PrimaryViewHiding) };
static Obj *new_civ(void) {
    Obj *o = obj_new(L"Windows.UI.ViewManagement.Core.CoreInputView", "CoreInputView");
    obj_add(o, &IID_ICIV, vt_CIV); obj_add(o, &IID_ICIV2, vt_CIV2);
    obj_add(o, &IID_ICIV3, vt_CIV3); obj_add(o, &IID_ICIV4, vt_CIV4);
    return o;
}
static HRESULT WINAPI CIVS_GetForCurrentView(Tear *s, void **out) {
    static Obj *g_civ;
    if (!g_civ) g_civ = new_civ();
    InterlockedIncrement(&g_civ->ref);
    *out = ITF(g_civ, 0); LOG("CoreInputViewStatics: GetForCurrentView -> %p", *out); return S_OK;
}
static HRESULT WINAPI CIVS2_GetForUIContext(Tear *s, void *ctx, void **out) {
    LOG("CoreInputViewStatics2: GetForUIContext(%p)", ctx); return CIVS_GetForCurrentView(s, out); }
static void *vt_CIVStatics[] = { IINSP, CIVS_GetForCurrentView };
static void *vt_CIVStatics2[] = { IINSP, CIVS2_GetForUIContext };
static Obj *factory_civ(void) {
    Obj *o = obj_new(L"Windows.UI.ViewManagement.Core.CoreInputView", "CoreInputViewFactory");
    obj_add(o, &IID_IActFactory, vt_AF_static);
    obj_add(o, &IID_ICIVStatics, vt_CIVStatics);
    obj_add(o, &IID_ICIVStatics2, vt_CIVStatics2);
    return o;
}

/* ================================================================
 * Windows.Foundation.Metadata.ApiInformation (overrides Wine's wintypes.dll version, whose
 * IsMethodPresent etc. return E_NOTIMPL and make the game throw hresult_not_implemented).
 * Policy: a type is present iff its runtime class is registered in the bottle; members of a present
 * type are present; only the base Universal/Foundation API contracts are present.
 * ================================================================ */
G(IID_IApiInfoStatics, 0x997439fe, 0xf681, 0x4a11, 0xb4,0x16,0xc1,0x3a,0x47,0xe8,0xba,0x36);
static BOOL type_registered(const WCHAR *t) {
    WCHAR key[512]; HKEY h;
    wsprintfW(key, L"Software\\Microsoft\\WindowsRuntime\\ActivatableClassId\\%s", t);
    if (RegOpenKeyExW(HKEY_LOCAL_MACHINE, key, 0, KEY_READ, &h)) return FALSE;
    RegCloseKey(h); return TRUE;
}
static HRESULT WINAPI AI_IsTypePresent(Tear *s, HSTRING t, BYTE *r) {
    *r = type_registered(hraw(t)); LOG("ApiInformation: IsTypePresent(%S) -> %d", hraw(t), *r); return S_OK; }
static HRESULT WINAPI AI_IsMember(Tear *s, HSTRING t, HSTRING m, BYTE *r) {
    *r = type_registered(hraw(t)); LOG("ApiInformation: Is*Present(%S, %S) -> %d", hraw(t), hraw(m), *r); return S_OK; }
static HRESULT WINAPI AI_IsMethodPresentWithArity(Tear *s, HSTRING t, HSTRING m, UINT32 n, BYTE *r) {
    *r = type_registered(hraw(t)); LOG("ApiInformation: IsMethodPresent(%S, %S, %u) -> %d", hraw(t), hraw(m), n, *r); return S_OK; }
static BOOL contract_present(const WCHAR *c, UINT16 major) {
    if (!lstrcmpW(c, L"Windows.Foundation.UniversalApiContract")) return major <= 14;
    if (!lstrcmpW(c, L"Windows.Foundation.FoundationContract")) return major <= 4;
    return FALSE;
}
static HRESULT WINAPI AI_IsContractMajor(Tear *s, HSTRING c, UINT16 major, BYTE *r) {
    *r = contract_present(hraw(c), major); LOG("ApiInformation: IsApiContractPresent(%S, %u) -> %d", hraw(c), major, *r); return S_OK; }
static HRESULT WINAPI AI_IsContractMajorMinor(Tear *s, HSTRING c, UINT16 major, UINT16 minor, BYTE *r) {
    *r = contract_present(hraw(c), major); LOG("ApiInformation: IsApiContractPresent(%S, %u.%u) -> %d", hraw(c), major, minor, *r); return S_OK; }
static void *vt_ApiInfoStatics[] = { IINSP, AI_IsTypePresent, AI_IsMember, AI_IsMethodPresentWithArity,
    AI_IsMember /*Event*/, AI_IsMember /*Property*/, AI_IsMember /*ReadOnlyProperty*/, AI_IsMember /*WriteableProperty*/,
    AI_IsMember /*EnumNamedValue*/, AI_IsContractMajor, AI_IsContractMajorMinor };
static Obj *factory_apiinfo(void) {
    Obj *o = obj_new(L"Windows.Foundation.Metadata.ApiInformation", "ApiInformationFactory");
    obj_add(o, &IID_IActFactory, vt_AF_static);
    obj_add(o, &IID_IApiInfoStatics, vt_ApiInfoStatics);
    return o;
}

/* ================================================================
 * exports
 * ================================================================ */
__declspec(dllexport) HRESULT WINAPI DllGetActivationFactory(HSTRING cls, void **factory) {
    init_combase();
    const WCHAR *n = hraw(cls);
    Obj *o = NULL;
    if (!lstrcmpW(n, L"Windows.ApplicationModel.DataTransfer.DataTransferManager")) o = factory_dtm();
    else if (!lstrcmpW(n, L"Windows.UI.Text.Core.CoreTextServicesManager")) o = factory_ctsm();
    else if (!lstrcmpW(n, L"Windows.Storage.StorageFile")) o = factory_storagefile();
    else if (!lstrcmpW(n, L"Windows.Storage.Pickers.FileSavePicker")) o = factory_fsp();
    else if (!lstrcmpW(n, L"Microsoft.Windows.Storage.Pickers.FileOpenPicker")) o = factory_sop();
    else if (!lstrcmpW(n, L"Windows.UI.ViewManagement.Core.CoreInputView")) o = factory_civ();
    else if (!lstrcmpW(n, L"Windows.Foundation.Metadata.ApiInformation")) o = factory_apiinfo();
    LOG("DllGetActivationFactory(%S) -> %p", n, o);
    if (!o) { *factory = NULL; return CLASS_E_CLASSNOTAVAILABLE; }
    *factory = ITF(o, 0);
    return S_OK;
}
__declspec(dllexport) HRESULT WINAPI DllCanUnloadNow(void) { return S_FALSE; }

BOOL WINAPI DllMain(HINSTANCE h, DWORD r, LPVOID p) {
    if (r == DLL_PROCESS_ATTACH) { InitializeCriticalSection(&g_cs); DisableThreadLibraryCalls(h); LOG("loaded"); }
    return TRUE;
}
