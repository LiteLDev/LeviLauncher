// XUser (provider 01acd177, iids 26f3c674 / cef4fac0; XUserDevice provider 7d824997) backed by a real
// Microsoft account + Xbox Live sign-in done inside the shim:
//   MSA device-code OAuth (login.live.com) -> Xbox device token (device.auth.xboxlive.com, ProofKey)
//   -> SISU authorize (sisu.xboxlive.com: user token + title token + XSTS for http://xboxlive.com, claims)
//   -> per-relying-party XSTS tokens (xsts.auth.xboxlive.com) for XUserGetTokenAndSignatureAsync.
// Design / env vars / reset: /Users/user/Bedrock/diagnostics/xuser.md
// Slot map: WineGDK include/xuser.idl order == /Users/user/Bedrock/diagnostics/gdk_thunks.md.
// Tokens are never logged (only lengths).
#include "xgr.h"
#include <stdlib.h>
#include <stdarg.h>
#include <stdint.h>
#include <ctype.h>
#include <winhttp.h>
#include <bcrypt.h>
#include <shellapi.h>

#define XU_PROV 0x01acd177UL
#define XUD_PROV 0x7d824997UL
#define IID_GAMERTAG 0xcef4fac0UL
#define TQ_PROV_ 0x073b7dcbUL
#define E_GU_NO_DEFAULT_USER ((HRESULT)0x89245106L)
#define E_GU_USER_NOT_FOUND ((HRESULT)0x89245104L)
#define E_GU_DEFERRAL_NOT_AVAILABLE ((HRESULT)0x89245103L)
#define E_GU_SIGNED_OUT ((HRESULT)0x89245101L)
#define E_GU_FAILED_TO_GET_TOKEN ((HRESULT)0x89245111L)
#define E_NOT_SUFFICIENT_BUFFER__ ((HRESULT)0x8007007AL)
#define A64 UINT64
#define IMPL(name) static UINT64 name(Obj* o, A64 a1, A64 a2, A64 a3, A64 a4, A64 a5, A64 a6)
#define IMPL9(name) static UINT64 name(Obj* o, A64 a1, A64 a2, A64 a3, A64 a4, A64 a5, A64 a6, A64 a7, A64 a8, A64 a9)

static const char* envs(const char* name, char* buf, DWORD n, const char* def) {
  DWORD r = GetEnvironmentVariableA(name, buf, n);
  if (r && r < n) return buf;
  return def;
}
static int envi(const char* name, int def) { char b[32]; const char* v = envs(name, b, sizeof b, NULL); return v ? (int)strtol(v, 0, 0) : def; }

// =============================================================== strings
typedef struct { char* p; size_t n, cap; } Buf;
static void b_add(Buf* b, const void* d, size_t n) {
  if (b->n + n + 1 > b->cap) {
    size_t c = b->cap ? b->cap : 256; while (c < b->n + n + 1) c *= 2;
    b->p = (char*)realloc(b->p, c); b->cap = c;
  }
  if (n) memcpy(b->p + b->n, d, n);
  b->n += n; b->p[b->n] = 0;
}
static void b_s(Buf* b, const char* s) { b_add(b, s, strlen(s)); }
static void b_f(Buf* b, const char* fmt, ...) {
  va_list ap; va_start(ap, fmt); int n = vsnprintf(NULL, 0, fmt, ap); va_end(ap);
  if (n <= 0) return;
  char* t = (char*)malloc(n + 1); va_start(ap, fmt); vsnprintf(t, n + 1, fmt, ap); va_end(ap);
  b_add(b, t, n); free(t);
}
static void b_free(Buf* b) { free(b->p); b->p = NULL; b->n = b->cap = 0; }
static void b_jstr(Buf* b, const char* s) {   // JSON string literal
  b_s(b, "\"");
  for (; s && *s; s++) {
    unsigned char c = (unsigned char)*s;
    if (c == '"' || c == '\\') { char t[2] = {'\\', (char)c}; b_add(b, t, 2); }
    else if (c < 0x20) b_f(b, "\\u%04x", c);
    else b_add(b, &c, 1);
  }
  b_s(b, "\"");
}
static void b_urlenc(Buf* b, const char* s) {  // application/x-www-form-urlencoded value
  for (; s && *s; s++) {
    unsigned char c = (unsigned char)*s;
    if ((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' || c == '_' || c == '.' || c == '~') b_add(b, &c, 1);
    else b_f(b, "%%%02X", c);
  }
}
static char* xstrdup(const char* s) { if (!s) return NULL; size_t n = strlen(s); char* r = (char*)malloc(n + 1); memcpy(r, s, n + 1); return r; }
static void setstr(char** dst, const char* s) { char* o = *dst; *dst = xstrdup(s); if (o) { SecureZeroMemory(o, strlen(o)); free(o); } }
static void lcpy(char* d, size_t n, const char* s) { if (!n) return; if (!s) s = ""; size_t l = strlen(s); if (l >= n) l = n - 1; memcpy(d, s, l); d[l] = 0; }

// =============================================================== base64
static const char B64[] = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
static char* b64enc(const BYTE* d, size_t n, int url) {
  char* o = (char*)malloc(n * 4 / 3 + 8); size_t j = 0;
  for (size_t i = 0; i < n; i += 3) {
    UINT32 v = (UINT32)d[i] << 16 | (i + 1 < n ? (UINT32)d[i + 1] << 8 : 0) | (i + 2 < n ? d[i + 2] : 0);
    o[j++] = B64[v >> 18 & 63]; o[j++] = B64[v >> 12 & 63];
    if (i + 1 < n) o[j++] = B64[v >> 6 & 63]; else if (!url) o[j++] = '=';
    if (i + 2 < n) o[j++] = B64[v & 63]; else if (!url) o[j++] = '=';
  }
  o[j] = 0;
  if (url) for (size_t k = 0; k < j; k++) { if (o[k] == '+') o[k] = '-'; else if (o[k] == '/') o[k] = '_'; }
  return o;
}
static BYTE* b64dec(const char* s, size_t* outn) {
  size_t n = strlen(s); BYTE* o = (BYTE*)malloc(n * 3 / 4 + 4); size_t j = 0; UINT32 v = 0; int bits = 0;
  for (size_t i = 0; i < n; i++) {
    char c = s[i]; int x;
    if (c >= 'A' && c <= 'Z') x = c - 'A'; else if (c >= 'a' && c <= 'z') x = c - 'a' + 26;
    else if (c >= '0' && c <= '9') x = c - '0' + 52; else if (c == '+' || c == '-') x = 62;
    else if (c == '/' || c == '_') x = 63; else continue;
    v = v << 6 | x; bits += 6;
    if (bits >= 8) { bits -= 8; o[j++] = (BYTE)(v >> bits); }
  }
  *outn = j; return o;
}

// =============================================================== JSON (minimal DOM)
enum { JNULL, JBOOL, JNUM, JSTR, JARR, JOBJ };
typedef struct J { int t; char* k; char* s; double num; int b; struct J *c, *nx; } J;
static void jfree(J* j) {
  while (j) { J* n = j->nx; jfree(j->c); if (j->s) { SecureZeroMemory(j->s, strlen(j->s)); free(j->s); } free(j->k); free(j); j = n; }
}
static void jws(const char** p) { while (**p == ' ' || **p == '\t' || **p == '\r' || **p == '\n') (*p)++; }
static int hex4(const char* p, UINT32* v) {
  *v = 0;
  for (int i = 0; i < 4; i++) {
    char c = p[i]; int x;
    if (c >= '0' && c <= '9') x = c - '0'; else if (c >= 'a' && c <= 'f') x = c - 'a' + 10; else if (c >= 'A' && c <= 'F') x = c - 'A' + 10; else return 0;
    *v = *v << 4 | x;
  }
  return 1;
}
static void utf8put(Buf* b, UINT32 cp) {
  char t[4]; int n;
  if (cp < 0x80) { t[0] = (char)cp; n = 1; }
  else if (cp < 0x800) { t[0] = (char)(0xC0 | cp >> 6); t[1] = (char)(0x80 | (cp & 63)); n = 2; }
  else if (cp < 0x10000) { t[0] = (char)(0xE0 | cp >> 12); t[1] = (char)(0x80 | (cp >> 6 & 63)); t[2] = (char)(0x80 | (cp & 63)); n = 3; }
  else { t[0] = (char)(0xF0 | cp >> 18); t[1] = (char)(0x80 | (cp >> 12 & 63)); t[2] = (char)(0x80 | (cp >> 6 & 63)); t[3] = (char)(0x80 | (cp & 63)); n = 4; }
  b_add(b, t, n);
}
static char* jpstr(const char** p) {
  if (**p != '"') return NULL;
  (*p)++; Buf b = {0}; b_add(&b, "", 0);
  while (**p && **p != '"') {
    char c = *(*p)++;
    if (c != '\\') { b_add(&b, &c, 1); continue; }
    c = *(*p)++;
    switch (c) {
    case 'n': b_s(&b, "\n"); break; case 't': b_s(&b, "\t"); break; case 'r': b_s(&b, "\r"); break;
    case 'b': b_s(&b, "\b"); break; case 'f': b_s(&b, "\f"); break;
    case 'u': {
      UINT32 cp; if (!hex4(*p, &cp)) { b_free(&b); return NULL; } *p += 4;
      if (cp >= 0xD800 && cp < 0xDC00 && (*p)[0] == '\\' && (*p)[1] == 'u') {
        UINT32 lo; if (hex4(*p + 2, &lo) && lo >= 0xDC00 && lo < 0xE000) { *p += 6; cp = 0x10000 + ((cp - 0xD800) << 10) + (lo - 0xDC00); }
      }
      utf8put(&b, cp); break; }
    case 0: b_free(&b); return NULL;
    default: b_add(&b, &c, 1);
    }
  }
  if (**p != '"') { b_free(&b); return NULL; }
  (*p)++; return b.p;
}
static J* jpval(const char** p, int depth) {
  if (depth > 64) return NULL;
  jws(p); J* j = (J*)calloc(1, sizeof *j);
  char c = **p;
  if (c == '{' || c == '[') {
    int obj = c == '{'; j->t = obj ? JOBJ : JARR; (*p)++; jws(p);
    J** tail = &j->c;
    if (**p == (obj ? '}' : ']')) { (*p)++; return j; }
    for (;;) {
      char* key = NULL;
      if (obj) { jws(p); key = jpstr(p); if (!key) goto bad; jws(p); if (**p != ':') { free(key); goto bad; } (*p)++; }
      J* v = jpval(p, depth + 1); if (!v) { free(key); goto bad; }
      v->k = key; *tail = v; tail = &v->nx;
      jws(p);
      if (**p == ',') { (*p)++; continue; }
      if (**p == (obj ? '}' : ']')) { (*p)++; return j; }
      goto bad;
    }
  }
  if (c == '"') { j->t = JSTR; j->s = jpstr(p); if (!j->s) goto bad; return j; }
  if (!strncmp(*p, "true", 4)) { j->t = JBOOL; j->b = 1; *p += 4; return j; }
  if (!strncmp(*p, "false", 5)) { j->t = JBOOL; *p += 5; return j; }
  if (!strncmp(*p, "null", 4)) { j->t = JNULL; *p += 4; return j; }
  if (c == '-' || (c >= '0' && c <= '9')) {
    char* e; j->t = JNUM; j->num = strtod(*p, &e); if (e == *p) goto bad;
    j->s = (char*)malloc(e - *p + 1); memcpy(j->s, *p, e - *p); j->s[e - *p] = 0; *p = e; return j;
  }
bad:
  jfree(j); return NULL;
}
static J* jparse(const char* s) { if (!s) return NULL; const char* p = s; J* j = jpval(&p, 0); return j; }
static J* jget(J* o, const char* k) {
  if (!o || o->t != JOBJ) return NULL;
  for (J* c = o->c; c; c = c->nx) if (c->k && !strcmp(c->k, k)) return c;
  return NULL;
}
static J* jat(J* a, int i) { if (!a || a->t != JARR) return NULL; for (J* c = a->c; c; c = c->nx) if (i-- == 0) return c; return NULL; }
static J* jpath(J* j, const char* path) {   // "a/b/0/c"
  char seg[64];
  while (j && *path) {
    const char* e = strchr(path, '/'); size_t n = e ? (size_t)(e - path) : strlen(path);
    if (n >= sizeof seg) return NULL;
    memcpy(seg, path, n); seg[n] = 0;
    if (j->t == JARR) j = jat(j, atoi(seg)); else j = jget(j, seg);
    path += n; if (*path == '/') path++;
  }
  return j;
}
static const char* jstr(J* j, const char* path) { j = jpath(j, path); return j && (j->t == JSTR || j->t == JNUM) ? j->s : NULL; }
static int jint(J* j, const char* path, int def) { j = jpath(j, path); return j && j->t == JNUM ? (int)j->num : (j && j->t == JSTR ? atoi(j->s) : def); }

// =============================================================== time
static INT64 now_unix(void) {
  FILETIME ft; GetSystemTimeAsFileTime(&ft);
  UINT64 v = (UINT64)ft.dwHighDateTime << 32 | ft.dwLowDateTime;
  return (INT64)(v / 10000000ULL) - 11644473600LL;
}
static UINT64 now_filetime(void) { FILETIME ft; GetSystemTimeAsFileTime(&ft); return (UINT64)ft.dwHighDateTime << 32 | ft.dwLowDateTime; }
static INT64 parse_iso(const char* s) {  // 2026-10-01T12:34:56.1234567Z -> unix seconds (0 on failure)
  SYSTEMTIME st = {0}; int Y, M, D, h, m, sec;
  if (!s || sscanf(s, "%d-%d-%dT%d:%d:%d", &Y, &M, &D, &h, &m, &sec) != 6) return 0;
  st.wYear = Y; st.wMonth = M; st.wDay = D; st.wHour = h; st.wMinute = m; st.wSecond = sec;
  FILETIME ft; if (!SystemTimeToFileTime(&st, &ft)) return 0;
  UINT64 v = (UINT64)ft.dwHighDateTime << 32 | ft.dwLowDateTime;
  return (INT64)(v / 10000000ULL) - 11644473600LL;
}

// =============================================================== HTTP (WinHTTP, synchronous)
typedef struct { DWORD status; Buf body; char xerr[64]; char wwwauth[256]; } Resp;
static HINTERNET g_sess;
static INIT_ONCE g_http_once = INIT_ONCE_STATIC_INIT;
static BOOL CALLBACK http_init(PINIT_ONCE o, PVOID p, PVOID* c) {
  g_sess = WinHttpOpen(L"XGR-XUser/1.0", WINHTTP_ACCESS_TYPE_DEFAULT_PROXY, WINHTTP_NO_PROXY_NAME, WINHTTP_NO_PROXY_BYPASS, 0);
  if (g_sess) {
    WinHttpSetTimeouts(g_sess, 15000, 15000, 30000, 30000);
    DWORD proto = WINHTTP_FLAG_SECURE_PROTOCOL_TLS1_2 | 0x00002000 /*TLS1_3*/;
    if (!WinHttpSetOption(g_sess, WINHTTP_OPTION_SECURE_PROTOCOLS, &proto, sizeof proto)) {
      proto = WINHTTP_FLAG_SECURE_PROTOCOL_TLS1_2; WinHttpSetOption(g_sess, WINHTTP_OPTION_SECURE_PROTOCOLS, &proto, sizeof proto);
    }
  } else xgr_log("XUSER: WinHttpOpen failed %lu", GetLastError());
  return TRUE;
}
static WCHAR* widen(const char* s) {
  int n = MultiByteToWideChar(CP_UTF8, 0, s, -1, NULL, 0); WCHAR* w = (WCHAR*)malloc(n * sizeof(WCHAR));
  MultiByteToWideChar(CP_UTF8, 0, s, -1, w, n); return w;
}
static char* narrow(const WCHAR* w) {
  if (!w) return NULL;
  int n = WideCharToMultiByte(CP_UTF8, 0, w, -1, NULL, 0, NULL, NULL); char* s = (char*)malloc(n ? n : 1); s[0] = 0;
  WideCharToMultiByte(CP_UTF8, 0, w, -1, s, n, NULL, NULL); return s;
}
// headers: "Name: value\r\n..." (may be NULL). Returns S_OK if a response (any status) was received.
static HRESULT http_req(const char* method, const char* url, const char* headers, const void* body, size_t blen, Resp* r) {
  InitOnceExecuteOnce(&g_http_once, http_init, NULL, NULL);
  memset(r, 0, sizeof *r);
  if (!g_sess) return E_FAIL;
  WCHAR* wurl = widen(url); WCHAR host[256] = {0}, path[4096] = {0};
  URL_COMPONENTS uc = { sizeof uc };
  uc.lpszHostName = host; uc.dwHostNameLength = 255; uc.lpszUrlPath = path; uc.dwUrlPathLength = 4000;
  WCHAR extra[4096] = {0}; uc.lpszExtraInfo = extra; uc.dwExtraInfoLength = 4000;
  HRESULT hr = E_FAIL; HINTERNET c = NULL, q = NULL;
  if (!WinHttpCrackUrl(wurl, 0, 0, &uc)) { xgr_log("XUSER: bad url %s (%lu)", url, GetLastError()); goto out; }
  wcscat(path, extra); if (!path[0]) wcscpy(path, L"/");
  c = WinHttpConnect(g_sess, host, uc.nPort, 0);
  if (!c) { hr = HRESULT_FROM_WIN32(GetLastError()); goto out; }
  WCHAR* wm = widen(method);
  q = WinHttpOpenRequest(c, wm, path, NULL, WINHTTP_NO_REFERER, WINHTTP_DEFAULT_ACCEPT_TYPES, uc.nScheme == INTERNET_SCHEME_HTTPS ? WINHTTP_FLAG_SECURE : 0);
  free(wm);
  if (!q) { hr = HRESULT_FROM_WIN32(GetLastError()); goto out; }
  WCHAR* wh = headers ? widen(headers) : NULL;
  BOOL ok = WinHttpSendRequest(q, wh ? wh : WINHTTP_NO_ADDITIONAL_HEADERS, wh ? (DWORD)-1L : 0, (LPVOID)body, (DWORD)blen, (DWORD)blen, 0);
  free(wh);
  if (!ok || !WinHttpReceiveResponse(q, NULL)) { hr = HRESULT_FROM_WIN32(GetLastError()); xgr_log("XUSER: HTTP %s %s failed (%lu)", method, url, GetLastError()); goto out; }
  DWORD st = 0, sl = sizeof st;
  WinHttpQueryHeaders(q, WINHTTP_QUERY_STATUS_CODE | WINHTTP_QUERY_FLAG_NUMBER, WINHTTP_HEADER_NAME_BY_INDEX, &st, &sl, WINHTTP_NO_HEADER_INDEX);
  r->status = st;
  WCHAR hv[256]; DWORD hl = sizeof hv;
  if (WinHttpQueryHeaders(q, WINHTTP_QUERY_CUSTOM, L"x-err", hv, &hl, WINHTTP_NO_HEADER_INDEX)) WideCharToMultiByte(CP_UTF8, 0, hv, -1, r->xerr, sizeof r->xerr, NULL, NULL);
  hl = sizeof hv;
  if (WinHttpQueryHeaders(q, WINHTTP_QUERY_WWW_AUTHENTICATE, WINHTTP_HEADER_NAME_BY_INDEX, hv, &hl, WINHTTP_NO_HEADER_INDEX)) WideCharToMultiByte(CP_UTF8, 0, hv, -1, r->wwwauth, sizeof r->wwwauth, NULL, NULL);
  b_add(&r->body, "", 0);
  for (;;) {
    DWORD avail = 0; if (!WinHttpQueryDataAvailable(q, &avail) || !avail) break;
    char* t = (char*)malloc(avail); DWORD got = 0;
    if (!WinHttpReadData(q, t, avail, &got)) { free(t); break; }
    b_add(&r->body, t, got); free(t);
    if (!got) break;
  }
  hr = S_OK;
out:
  if (q) WinHttpCloseHandle(q);
  if (c) WinHttpCloseHandle(c);
  free(wurl); return hr;
}
static void resp_free(Resp* r) { if (r->body.p) SecureZeroMemory(r->body.p, r->body.n); b_free(&r->body); }
// log an error response; bodies of error responses carry no tokens, but cap and strip anyway
static void log_err(const char* what, HRESULT hr, Resp* r) {
  char snip[301] = {0};
  if (r->body.p) { lcpy(snip, sizeof snip, r->body.p); for (char* p = snip; *p; p++) if (*p == '\r' || *p == '\n') *p = ' '; }
  xgr_log("XUSER: %s failed hr=%08lx http=%lu x-err=%s www-auth=%s body=%s", what, (unsigned long)hr, r->status, r->xerr[0] ? r->xerr : "-", r->wwwauth[0] ? r->wwwauth : "-", snip);
}

// =============================================================== crypto (BCrypt ECDSA P-256 + SHA-256)
static BCRYPT_ALG_HANDLE g_alg_ec, g_alg_sha;
static BCRYPT_KEY_HANDLE g_key;
static BYTE g_pubxy[64];
static HRESULT crypto_init(void) {
  if (g_alg_ec) return S_OK;
  NTSTATUS s = BCryptOpenAlgorithmProvider(&g_alg_ec, BCRYPT_ECDSA_P256_ALGORITHM, NULL, 0);
  if (s) { xgr_log("XUSER: ECDSA provider %08lx", s); g_alg_ec = NULL; return E_FAIL; }
  s = BCryptOpenAlgorithmProvider(&g_alg_sha, BCRYPT_SHA256_ALGORITHM, NULL, 0);
  if (s) { xgr_log("XUSER: SHA256 provider %08lx", s); return E_FAIL; }
  return S_OK;
}
static HRESULT key_read_pub(void) {
  BYTE blob[8 + 64]; ULONG got = 0;
  NTSTATUS s = BCryptExportKey(g_key, NULL, BCRYPT_ECCPUBLIC_BLOB, blob, sizeof blob, &got, 0);
  if (s || got != sizeof blob) { xgr_log("XUSER: export pub %08lx got=%lu", s, got); return E_FAIL; }
  memcpy(g_pubxy, blob + 8, 64); return S_OK;
}
static HRESULT key_generate(void) {
  if (FAILED(crypto_init())) return E_FAIL;
  BCRYPT_KEY_HANDLE k = NULL;
  NTSTATUS s = BCryptGenerateKeyPair(g_alg_ec, &k, 256, 0);
  if (!s) s = BCryptFinalizeKeyPair(k, 0);
  if (s) { xgr_log("XUSER: keygen %08lx", s); return E_FAIL; }
  if (g_key) BCryptDestroyKey(g_key);
  g_key = k; return key_read_pub();
}
static char* key_export_b64(void) {   // BCRYPT_ECCPRIVATE_BLOB, base64
  BYTE blob[8 + 96]; ULONG got = 0;
  NTSTATUS s = BCryptExportKey(g_key, NULL, BCRYPT_ECCPRIVATE_BLOB, blob, sizeof blob, &got, 0);
  if (s) { xgr_log("XUSER: export priv %08lx", s); return NULL; }
  char* r = b64enc(blob, got, 0); SecureZeroMemory(blob, sizeof blob); return r;
}
static HRESULT key_import_b64(const char* b64) {
  if (FAILED(crypto_init()) || !b64) return E_FAIL;
  size_t n; BYTE* blob = b64dec(b64, &n); BCRYPT_KEY_HANDLE k = NULL;
  NTSTATUS s = BCryptImportKeyPair(g_alg_ec, NULL, BCRYPT_ECCPRIVATE_BLOB, &k, blob, (ULONG)n, 0);
  SecureZeroMemory(blob, n); free(blob);
  if (s) { xgr_log("XUSER: import key %08lx", s); return E_FAIL; }
  if (g_key) BCryptDestroyKey(g_key);
  g_key = k; return key_read_pub();
}
static HRESULT sha256(const void* d, size_t n, BYTE out[32]) {
  BCRYPT_HASH_HANDLE h = NULL;
  if (FAILED(crypto_init())) return E_FAIL;
  NTSTATUS s = BCryptCreateHash(g_alg_sha, &h, NULL, 0, NULL, 0, 0);
  if (!s) s = BCryptHashData(h, (PUCHAR)d, (ULONG)n, 0);
  if (!s) s = BCryptFinishHash(h, out, 32, 0);
  if (h) BCryptDestroyHash(h);
  return s ? E_FAIL : S_OK;
}
static HRESULT ec_sign(const BYTE hash[32], BYTE sig[64]) {
  ULONG got = 0;
  NTSTATUS s = BCryptSignHash(g_key, NULL, (PUCHAR)hash, 32, sig, 64, &got, 0);
  if (s || got != 64) { xgr_log("XUSER: sign %08lx got=%lu", s, got); return E_FAIL; }
  return S_OK;
}
static void proofkey_json(Buf* b) {
  char* x = b64enc(g_pubxy, 32, 1); char* y = b64enc(g_pubxy + 32, 32, 1);
  b_f(b, "{\"crv\":\"P-256\",\"alg\":\"ES256\",\"use\":\"sig\",\"kty\":\"EC\",\"x\":\"%s\",\"y\":\"%s\"}", x, y);
  free(x); free(y);
}

// Xbox Live request signature (policy version 1):
//   signed = BE32(1) 00 BE64(filetime) 00 METHOD 00 path?query 00 Authorization 00 body[:maxBody] 00
//   header = base64( BE32(1) || BE64(filetime) || r||s )
static const char* url_path(const char* url) {
  const char* p = strstr(url, "://"); p = p ? p + 3 : url;
  p = strpbrk(p, "/?"); return p ? p : "";
}
static char* xbl_signature_at(UINT64 ts, const char* method, const char* url, const char* auth, const void* body, size_t blen, size_t maxbody) {
  Buf m = {0}; BYTE hdr[4] = {0, 0, 0, 1}, z = 0, tsb[8];
  for (int i = 0; i < 8; i++) tsb[i] = (BYTE)(ts >> (56 - 8 * i));
  b_add(&m, hdr, 4); b_add(&m, &z, 1); b_add(&m, tsb, 8); b_add(&m, &z, 1);
  b_s(&m, method); b_add(&m, &z, 1);
  const char* path = url_path(url); const char* hash = strchr(path, '#');
  if (!*path || *path == '?') b_s(&m, "/");
  b_add(&m, path, hash ? (size_t)(hash - path) : strlen(path)); b_add(&m, &z, 1);
  b_s(&m, auth ? auth : ""); b_add(&m, &z, 1);
  if (body && blen) b_add(&m, body, blen < maxbody ? blen : maxbody);
  b_add(&m, &z, 1);
  BYTE h[32], out[4 + 8 + 64]; char* r = NULL;
  if (SUCCEEDED(sha256(m.p, m.n, h)) && SUCCEEDED(ec_sign(h, out + 12))) {
    memcpy(out, hdr, 4); memcpy(out + 4, tsb, 8); r = b64enc(out, sizeof out, 0);
  }
  b_free(&m); return r;
}
static char* xbl_signature(const char* method, const char* url, const char* auth, const void* body, size_t blen, size_t maxbody) {
  return xbl_signature_at(now_filetime(), method, url, auth, body, blen, maxbody);
}

// =============================================================== auth state
#define MAX_XSTS 48
typedef struct { char* rp; char* token; char uhs[64]; INT64 exp; } Xsts;
static struct {
  CRITICAL_SECTION cs; int loaded;
  char client_id[64], scope[128], device_id[48], sandbox[32], devtype[32], devver[32], title_id[16];
  char* msa_refresh; char* msa_access; INT64 msa_exp;
  char* dev_token; INT64 dev_exp;
  char* user_token; INT64 user_exp;
  char* title_token; INT64 title_exp;
  Xsts xsts[MAX_XSTS]; int nxsts;
  char uhs[64], xid[32], gtg[64], mgt[64], mgs[32], umg[128], agg[16]; char* prv;
  volatile LONG signed_in, interactive;
} A;
static INIT_ONCE g_auth_once = INIT_ONCE_STATIC_INIT;
static BOOL CALLBACK auth_init(PINIT_ONCE o, PVOID p, PVOID* c) { InitializeCriticalSection(&A.cs); return TRUE; }
static void auth_lock(void) { InitOnceExecuteOnce(&g_auth_once, auth_init, NULL, NULL); EnterCriticalSection(&A.cs); }
static void auth_unlock(void) { LeaveCriticalSection(&A.cs); }

static void auth_path(char* out, size_t n) {
  char b[MAX_PATH];
  if (envs("XGR_AUTH_FILE", b, sizeof b, NULL)) { lcpy(out, n, b); return; }
  char ad[MAX_PATH]; const char* app = envs("APPDATA", ad, sizeof ad, NULL);
  if (!app) { lcpy(out, n, "C:\\xgr_auth.json"); return; }
  snprintf(out, n, "%s\\xgr", app); CreateDirectoryA(out, NULL);
  snprintf(out, n, "%s\\xgr\\auth.json", app);
}
static void gen_uuid(char* out) {
  BYTE r[16]; BCryptGenRandom(NULL, r, 16, BCRYPT_USE_SYSTEM_PREFERRED_RNG);
  r[6] = (r[6] & 0x0F) | 0x40; r[8] = (r[8] & 0x3F) | 0x80;
  sprintf(out, "{%02X%02X%02X%02X-%02X%02X-%02X%02X-%02X%02X-%02X%02X%02X%02X%02X%02X}", r[0], r[1], r[2], r[3], r[4], r[5], r[6], r[7], r[8], r[9], r[10], r[11], r[12], r[13], r[14], r[15]);
}
static void title_id_from_config(char* out, size_t n) {
  lcpy(out, n, "35760C07");
  char exe[MAX_PATH]; DWORD l = GetModuleFileNameA(NULL, exe, MAX_PATH); if (!l || l >= MAX_PATH) return;
  char* s = strrchr(exe, '\\'); if (!s) return; strcpy(s + 1, "MicrosoftGame.Config");
  FILE* f = fopen(exe, "rb"); if (!f) return;
  char buf[16384]; size_t got = fread(buf, 1, sizeof buf - 1, f); fclose(f); buf[got] = 0;
  char* t = strstr(buf, "<TitleId>"); if (!t) return; t += 9;
  char* e = strstr(t, "</TitleId>"); if (!e || e - t >= (int)n || e - t > 12) return;
  memcpy(out, t, e - t); out[e - t] = 0;
}
static void auth_save(void) {   // caller holds lock
  char path[MAX_PATH], tmp[MAX_PATH + 8]; auth_path(path, sizeof path); snprintf(tmp, sizeof tmp, "%s.tmp", path);
  char* key = key_export_b64();
  Buf b = {0};
  b_s(&b, "{\n  \"version\": 1,\n  \"client_id\": "); b_jstr(&b, A.client_id);
  b_s(&b, ",\n  \"device_id\": "); b_jstr(&b, A.device_id);
  b_s(&b, ",\n  \"device_key\": "); b_jstr(&b, key ? key : "");
  b_s(&b, ",\n  \"msa_refresh_token\": "); b_jstr(&b, A.msa_refresh ? A.msa_refresh : "");
  b_s(&b, ",\n  \"xuid\": "); b_jstr(&b, A.xid);
  b_s(&b, ",\n  \"gamertag\": "); b_jstr(&b, A.gtg);
  b_s(&b, "\n}\n");
  FILE* f = fopen(tmp, "wb");
  if (f) { fwrite(b.p, 1, b.n, f); fclose(f); if (!MoveFileExA(tmp, path, MOVEFILE_REPLACE_EXISTING)) xgr_log("XUSER: save %s failed %lu", path, GetLastError()); else xgr_log("XUSER: saved credentials to %s (refresh token %s)", path, A.msa_refresh ? "present" : "none"); }
  else xgr_log("XUSER: cannot write %s", tmp);
  SecureZeroMemory(b.p, b.n); b_free(&b);
  if (key) { SecureZeroMemory(key, strlen(key)); free(key); }
}
static void auth_load(void) {   // caller holds lock; idempotent
  if (A.loaded) return;
  A.loaded = 1;
  char b[256];
  lcpy(A.client_id, sizeof A.client_id, envs("XGR_MSA_CLIENT_ID", b, sizeof b, "0000000048183522"));
  lcpy(A.scope, sizeof A.scope, envs("XGR_MSA_SCOPE", b, sizeof b, "service::user.auth.xboxlive.com::MBI_SSL"));
  lcpy(A.sandbox, sizeof A.sandbox, envs("XGR_SANDBOX", b, sizeof b, "RETAIL"));
  lcpy(A.devtype, sizeof A.devtype, envs("XGR_DEVICE_TYPE", b, sizeof b, "Android"));
  lcpy(A.devver, sizeof A.devver, envs("XGR_DEVICE_VERSION", b, sizeof b, "10"));
  title_id_from_config(A.title_id, sizeof A.title_id);
  char path[MAX_PATH]; auth_path(path, sizeof path);
  FILE* f = fopen(path, "rb"); J* j = NULL;
  if (f) {
    Buf fb = {0}; char t[4096]; size_t n;
    while ((n = fread(t, 1, sizeof t, f)) > 0) b_add(&fb, t, n);
    fclose(f); j = jparse(fb.p); if (fb.p) SecureZeroMemory(fb.p, fb.n); b_free(&fb);
  }
  if (j) {
    const char* key = jstr(j, "device_key"); const char* did = jstr(j, "device_id");
    if (key && *key && SUCCEEDED(key_import_b64(key)) && did && *did) lcpy(A.device_id, sizeof A.device_id, did);
    const char* cid = jstr(j, "client_id"); const char* rt = jstr(j, "msa_refresh_token");
    if (cid && !strcmp(cid, A.client_id) && rt && *rt) setstr(&A.msa_refresh, rt);
    else if (rt && *rt) xgr_log("XUSER: stored refresh token is for client %s, current client %s -> ignored", cid ? cid : "?", A.client_id);
    lcpy(A.gtg, sizeof A.gtg, jstr(j, "gamertag"));
    jfree(j);
  }
  if (!g_key || !A.device_id[0]) { key_generate(); gen_uuid(A.device_id); }
  xgr_log("XUSER: auth file %s: %s, client_id=%s title=%s", path, A.msa_refresh ? "cached sign-in present" : "no cached sign-in", A.client_id, A.title_id);
}
static void clear_user_tokens(void) {
  setstr(&A.user_token, NULL); setstr(&A.title_token, NULL); A.user_exp = A.title_exp = 0;
  for (int i = 0; i < A.nxsts; i++) { setstr(&A.xsts[i].token, NULL); A.xsts[i].exp = 0; }
}

// =============================================================== MSA
static HRESULT msa_post(const char* url, Buf* form, J** out, Resp* r) {
  HRESULT hr = http_req("POST", url, "Content-Type: application/x-www-form-urlencoded\r\nAccept: application/json\r\n", form->p, form->n, r);
  *out = SUCCEEDED(hr) ? jparse(r->body.p) : NULL;
  return hr;
}
static void msa_take_tokens(J* j) {
  setstr(&A.msa_access, jstr(j, "access_token"));
  const char* rt = jstr(j, "refresh_token"); if (rt && *rt) setstr(&A.msa_refresh, rt);
  A.msa_exp = now_unix() + jint(j, "expires_in", 3600);
}
static HRESULT msa_refresh(void) {
  if (!A.msa_refresh) return E_GU_NO_DEFAULT_USER;
  Buf f = {0}; Resp r; J* j = NULL;
  b_s(&f, "client_id="); b_urlenc(&f, A.client_id);
  b_s(&f, "&scope="); b_urlenc(&f, A.scope);
  b_s(&f, "&grant_type=refresh_token&refresh_token="); b_urlenc(&f, A.msa_refresh);
  HRESULT hr = msa_post("https://login.live.com/oauth20_token.srf", &f, &j, &r);
  SecureZeroMemory(f.p, f.n); b_free(&f);
  if (SUCCEEDED(hr) && r.status == 200 && jstr(j, "access_token")) {
    msa_take_tokens(j); xgr_log("XUSER: MSA refresh ok (access token %u bytes)", (unsigned)strlen(A.msa_access)); auth_save();
  } else {
    const char* err = jstr(j, "error");
    log_err("MSA refresh", hr, &r);
    if (err && (!strcmp(err, "invalid_grant") || !strcmp(err, "unauthorized_client"))) {
      xgr_log("XUSER: refresh token rejected (%s) -> forgetting cached sign-in", err);
      setstr(&A.msa_refresh, NULL); auth_save();
      hr = E_GU_NO_DEFAULT_USER;
    } else hr = FAILED(hr) ? hr : E_GU_FAILED_TO_GET_TOKEN;
  }
  jfree(j); resp_free(&r); return hr;
}

// ---- sign-in prompt (MessageBox on its own thread + log + C:\xgr_signin.txt)
static volatile LONG g_prompt_cancel, g_prompt_done; static volatile DWORD g_prompt_tid;
typedef struct { WCHAR text[1024]; } PromptCtx;
static DWORD WINAPI prompt_thread(LPVOID p) {
  PromptCtx* c = (PromptCtx*)p;
  g_prompt_tid = GetCurrentThreadId();
  int r = MessageBoxW(NULL, c->text, L"Minecraft - Microsoft account sign-in", MB_OKCANCEL | MB_ICONINFORMATION | MB_SETFOREGROUND | MB_TOPMOST);
  if (r == IDCANCEL && !g_prompt_done) { InterlockedExchange(&g_prompt_cancel, 1); xgr_log("XUSER: sign-in cancelled from the dialog"); }
  g_prompt_tid = 0; free(c); return 0;
}
static BOOL CALLBACK close_wnd(HWND h, LPARAM l) { PostMessageW(h, WM_CLOSE, 0, 0); return TRUE; }
static void prompt_close(void) {
  InterlockedExchange(&g_prompt_done, 1);
  DWORD tid = g_prompt_tid; if (tid) EnumThreadWindows(tid, close_wnd, 0);
}
static void signin_file(const char* text) {
  FILE* f = fopen("C:\\xgr_signin.txt", "wb"); if (f) { fputs(text, f); fclose(f); }
}
static void prompt_show(const char* uri, const char* code, int expires) {
  xgr_log("SIGNIN CODE: %s URL: %s (expires in %d s)", code, uri, expires);
  fprintf(stderr, "[xgr] SIGNIN CODE: %s URL: %s\n", code, uri); fflush(stderr);
  char t[512];
  snprintf(t, sizeof t, "Microsoft account sign-in for Minecraft\r\n\r\nGo to %s and enter the code:\r\n\r\n    %s\r\n\r\n(code expires in %d minutes)\r\n", uri, code, expires / 60);
  signin_file(t);
  InterlockedExchange(&g_prompt_cancel, 0); InterlockedExchange(&g_prompt_done, 0);
  if (!envi("XGR_SIGNIN_NOMSGBOX", 0)) {
    PromptCtx* c = (PromptCtx*)calloc(1, sizeof *c);
    snprintf(t, sizeof t, "To sign in with your Microsoft account, open\n\n%s\n\nin a web browser and enter the code\n\n%s\n\nThis window can be closed with OK; the game keeps waiting for up to %d minutes.\nPress Cancel to abort the sign-in.", uri, code, expires / 60);
    MultiByteToWideChar(CP_UTF8, 0, t, -1, c->text, 1024);
    HANDLE th = CreateThread(NULL, 0, prompt_thread, c, 0, NULL); if (th) CloseHandle(th); else free(c);
  }
  if (envi("XGR_SIGNIN_OPEN_BROWSER", 0)) {
    char u[256]; snprintf(u, sizeof u, "%s?otc=%s", uri, code);
    ShellExecuteA(NULL, "open", u, NULL, NULL, SW_SHOWNORMAL);
  }
}

// Device-code flow: returns S_OK once A.msa_access/A.msa_refresh are set. cancel: polled every second.
static HRESULT msa_device_flow(volatile LONG* cancel, char* code_out, size_t code_n) {
  Buf f = {0}; Resp r; J* j = NULL;
  b_s(&f, "client_id="); b_urlenc(&f, A.client_id);
  b_s(&f, "&scope="); b_urlenc(&f, A.scope);
  b_s(&f, "&response_type=device_code");
  HRESULT hr = msa_post("https://login.live.com/oauth20_connect.srf", &f, &j, &r); b_free(&f);
  const char* dc = jstr(j, "device_code"); const char* uc = jstr(j, "user_code");
  if (FAILED(hr) || r.status != 200 || !dc || !uc) { log_err("MSA device code request", hr, &r); jfree(j); resp_free(&r); return FAILED(hr) ? hr : E_FAIL; }
  char* device_code = xstrdup(dc); char user_code[64]; lcpy(user_code, sizeof user_code, uc);
  if (code_out) lcpy(code_out, code_n, user_code);
  const char* vu = jstr(j, "verification_uri"); char uri[256]; lcpy(uri, sizeof uri, vu ? vu : "https://www.microsoft.com/link");
  int interval = jint(j, "interval", 5), expires = jint(j, "expires_in", 900);
  int limit = envi("XGR_SIGNIN_TIMEOUT", expires); if (limit > expires) limit = expires;
  jfree(j); resp_free(&r);
  prompt_show(uri, user_code, expires);
  if (envi("XGR_SIGNIN_CODE_ONLY", 0)) { free(device_code); return S_FALSE; }  // test hook: stop after the code
  ULONGLONG deadline = GetTickCount64() + (ULONGLONG)limit * 1000; hr = E_ABORT;
  for (;;) {
    for (int i = 0; i < interval * 10; i++) { if (*cancel || g_prompt_cancel) goto done; Sleep(100); }
    if (GetTickCount64() > deadline) { xgr_log("XUSER: device code expired without sign-in"); goto done; }
    b_s(&f, "client_id="); b_urlenc(&f, A.client_id);
    b_s(&f, "&grant_type=urn%3Aietf%3Aparams%3Aoauth%3Agrant-type%3Adevice_code&device_code="); b_urlenc(&f, device_code);
    HRESULT h2 = msa_post("https://login.live.com/oauth20_token.srf", &f, &j, &r); b_free(&f);
    if (SUCCEEDED(h2) && r.status == 200 && jstr(j, "access_token")) {
      msa_take_tokens(j); jfree(j); resp_free(&r);
      xgr_log("XUSER: MSA device-code sign-in completed (refresh token %s)", A.msa_refresh ? "received" : "MISSING");
      hr = S_OK; goto done;
    }
    const char* err = jstr(j, "error");
    if (err && !strcmp(err, "authorization_pending")) { }
    else if (err && !strcmp(err, "slow_down")) interval += 5;
    else if (err) { log_err("MSA device-code poll", h2, &r); jfree(j); resp_free(&r); hr = E_ABORT; goto done; }
    else log_err("MSA device-code poll (transient)", h2, &r);
    jfree(j); j = NULL; resp_free(&r);
  }
done:
  prompt_close();
  SecureZeroMemory(device_code, strlen(device_code)); free(device_code);
  return hr;
}

// =============================================================== Xbox Live
static HRESULT xbl_post(const char* url, Buf* body, const char* contract, J** out, Resp* r) {
  char* sig = xbl_signature("POST", url, "", body->p, body->n, 8192);
  if (!sig) return E_FAIL;
  Buf h = {0};
  b_f(&h, "Content-Type: application/json\r\nAccept: application/json\r\nx-xbl-contract-version: %s\r\nSignature: %s\r\n", contract, sig);
  free(sig);
  HRESULT hr = http_req("POST", url, h.p, body->p, body->n, r);
  b_free(&h);
  *out = SUCCEEDED(hr) && r->status == 200 ? jparse(r->body.p) : NULL;
  return hr;
}
static HRESULT xbl_device_auth(void) {
  Buf b = {0}; Resp r; J* j = NULL;
  b_s(&b, "{\"Properties\":{\"AuthMethod\":\"ProofOfPossession\",\"Id\":"); b_jstr(&b, A.device_id);
  b_s(&b, ",\"DeviceType\":"); b_jstr(&b, A.devtype);
  b_s(&b, ",\"SerialNumber\":"); b_jstr(&b, A.device_id);
  b_s(&b, ",\"Version\":"); b_jstr(&b, A.devver);
  b_s(&b, ",\"ProofKey\":"); proofkey_json(&b);
  b_s(&b, "},\"RelyingParty\":\"http://auth.xboxlive.com\",\"TokenType\":\"JWT\"}");
  HRESULT hr = xbl_post("https://device.auth.xboxlive.com/device/authenticate", &b, "1", &j, &r); b_free(&b);
  const char* tok = jstr(j, "Token");
  if (SUCCEEDED(hr) && tok) {
    setstr(&A.dev_token, tok); A.dev_exp = parse_iso(jstr(j, "NotAfter")); if (!A.dev_exp) A.dev_exp = now_unix() + 3600;
    xgr_log("XUSER: device token ok (%u bytes, did=%s)", (unsigned)strlen(tok), jstr(j, "DisplayClaims/xdi/did") ? jstr(j, "DisplayClaims/xdi/did") : "?");
    hr = S_OK;
  } else { log_err("device authenticate", hr, &r); hr = FAILED(hr) ? hr : E_GU_FAILED_TO_GET_TOKEN; }
  jfree(j); resp_free(&r); return hr;
}
static void apply_claims(J* xui) {
  if (!xui) return;
  const char* v;
  if ((v = jstr(xui, "uhs"))) lcpy(A.uhs, sizeof A.uhs, v);
  if ((v = jstr(xui, "xid"))) lcpy(A.xid, sizeof A.xid, v);
  if ((v = jstr(xui, "gtg"))) lcpy(A.gtg, sizeof A.gtg, v);
  if ((v = jstr(xui, "mgt"))) lcpy(A.mgt, sizeof A.mgt, v);
  if ((v = jstr(xui, "mgs"))) lcpy(A.mgs, sizeof A.mgs, v);
  if ((v = jstr(xui, "umg"))) lcpy(A.umg, sizeof A.umg, v);
  if ((v = jstr(xui, "agg"))) lcpy(A.agg, sizeof A.agg, v);
  if ((v = jstr(xui, "prv"))) setstr(&A.prv, v);
}
static Xsts* xsts_slot(const char* rp) {
  for (int i = 0; i < A.nxsts; i++) if (!strcmp(A.xsts[i].rp, rp)) return &A.xsts[i];
  if (A.nxsts >= MAX_XSTS) { setstr(&A.xsts[0].token, NULL); A.xsts[0].exp = 0; setstr(&A.xsts[0].rp, rp); return &A.xsts[0]; }
  Xsts* x = &A.xsts[A.nxsts++]; memset(x, 0, sizeof *x); x->rp = xstrdup(rp); return x;
}
static void xsts_store(const char* rp, J* tokobj) {
  Xsts* x = xsts_slot(rp);
  setstr(&x->token, jstr(tokobj, "Token"));
  x->exp = parse_iso(jstr(tokobj, "NotAfter")); if (!x->exp) x->exp = now_unix() + 3600;
  const char* u = jstr(tokobj, "DisplayClaims/xui/0/uhs"); lcpy(x->uhs, sizeof x->uhs, u ? u : A.uhs);
}
static HRESULT xbl_sisu(void) {
  Buf b = {0}; Resp r; J* j = NULL;
  b_s(&b, "{\"AccessToken\":\"t="); b_s(&b, A.msa_access); b_s(&b, "\"");
  b_s(&b, ",\"AppId\":"); b_jstr(&b, A.client_id);
  b_s(&b, ",\"DeviceToken\":"); b_jstr(&b, A.dev_token);
  b_s(&b, ",\"Sandbox\":"); b_jstr(&b, A.sandbox);
  b_s(&b, ",\"UseModernGamertag\":true,\"SiteName\":\"user.auth.xboxlive.com\",\"RelyingParty\":\"http://xboxlive.com\",\"ProofKey\":"); proofkey_json(&b);
  b_s(&b, "}");
  HRESULT hr = xbl_post("https://sisu.xboxlive.com/authorize", &b, "1", &j, &r);
  SecureZeroMemory(b.p, b.n); b_free(&b);
  const char* ut = jstr(j, "UserToken/Token"); const char* az = jstr(j, "AuthorizationToken/Token");
  if (SUCCEEDED(hr) && ut && az) {
    setstr(&A.user_token, ut); A.user_exp = parse_iso(jstr(j, "UserToken/NotAfter")); if (!A.user_exp) A.user_exp = now_unix() + 3600;
    const char* tt = jstr(j, "TitleToken/Token");
    setstr(&A.title_token, tt); A.title_exp = tt ? parse_iso(jstr(j, "TitleToken/NotAfter")) : 0;
    apply_claims(jpath(j, "AuthorizationToken/DisplayClaims/xui/0"));
    xsts_store("http://xboxlive.com", jpath(j, "AuthorizationToken"));
    xgr_log("XUSER: SISU ok: user token %u bytes, title token %s, xsts %u bytes, xuid=%s gamertag=%s agg=%s",
            (unsigned)strlen(ut), tt ? "yes" : "no", (unsigned)strlen(az), A.xid, A.gtg, A.agg);
    hr = S_OK;
  } else { log_err("SISU authorize", hr, &r); hr = FAILED(hr) ? hr : E_GU_FAILED_TO_GET_TOKEN; }
  jfree(j); resp_free(&r); return hr;
}
static HRESULT xbl_user_auth(void) {   // fallback when SISU is refused
  Buf b = {0}; Resp r; J* j = NULL;
  b_s(&b, "{\"RelyingParty\":\"http://auth.xboxlive.com\",\"TokenType\":\"JWT\",\"Properties\":{\"AuthMethod\":\"RPS\",\"SiteName\":\"user.auth.xboxlive.com\",\"RpsTicket\":\"t=");
  b_s(&b, A.msa_access); b_s(&b, "\",\"ProofKey\":"); proofkey_json(&b); b_s(&b, "}}");
  HRESULT hr = xbl_post("https://user.auth.xboxlive.com/user/authenticate", &b, "2", &j, &r);
  SecureZeroMemory(b.p, b.n); b_free(&b);
  const char* ut = jstr(j, "Token");
  if (SUCCEEDED(hr) && ut) {
    setstr(&A.user_token, ut); A.user_exp = parse_iso(jstr(j, "NotAfter")); if (!A.user_exp) A.user_exp = now_unix() + 3600;
    setstr(&A.title_token, NULL);
    const char* u = jstr(j, "DisplayClaims/xui/0/uhs"); if (u) lcpy(A.uhs, sizeof A.uhs, u);
    xgr_log("XUSER: user.auth ok (fallback, no title token)"); hr = S_OK;
  } else { log_err("user authenticate", hr, &r); hr = FAILED(hr) ? hr : E_GU_FAILED_TO_GET_TOKEN; }
  jfree(j); resp_free(&r); return hr;
}
static HRESULT xbl_xsts(const char* rp) {
  Buf b = {0}; Resp r; J* j = NULL;
  b_s(&b, "{\"Properties\":{\"SandboxId\":"); b_jstr(&b, A.sandbox);
  b_s(&b, ",\"DeviceToken\":"); b_jstr(&b, A.dev_token);
  if (A.title_token) { b_s(&b, ",\"TitleToken\":"); b_jstr(&b, A.title_token); }
  b_s(&b, ",\"UserTokens\":["); b_jstr(&b, A.user_token);
  b_s(&b, "]},\"RelyingParty\":"); b_jstr(&b, rp); b_s(&b, ",\"TokenType\":\"JWT\"}");
  HRESULT hr = xbl_post("https://xsts.auth.xboxlive.com/xsts/authorize", &b, "1", &j, &r);
  SecureZeroMemory(b.p, b.n); b_free(&b);
  if (SUCCEEDED(hr) && jstr(j, "Token")) {
    xsts_store(rp, j);
    if (!strcmp(rp, "http://xboxlive.com")) apply_claims(jpath(j, "DisplayClaims/xui/0"));
    xgr_log("XUSER: XSTS ok for %s (%u bytes)", rp, (unsigned)strlen(jstr(j, "Token")));
    hr = S_OK;
  } else { log_err("XSTS authorize", hr, &r); if (SUCCEEDED(hr)) hr = E_GU_FAILED_TO_GET_TOKEN; }
  jfree(j); resp_free(&r); return hr;
}
// Make sure device token + user token are valid (refreshing via MSA refresh token). Caller holds lock.
static HRESULT ensure_chain(void) {
  INT64 now = now_unix(); HRESULT hr;
  if (!A.dev_token || A.dev_exp < now + 300) if (FAILED(hr = xbl_device_auth())) return hr;
  if (!A.user_token || A.user_exp < now + 300 || (A.title_token && A.title_exp && A.title_exp < now + 300)) {
    if (!A.msa_access || A.msa_exp < now + 120) if (FAILED(hr = msa_refresh())) return hr;
    clear_user_tokens();
    hr = xbl_sisu();
    if (FAILED(hr) && !envi("XGR_NO_USERAUTH_FALLBACK", 0)) {
      if (SUCCEEDED(xbl_user_auth())) hr = xbl_xsts("http://xboxlive.com");
    }
    if (FAILED(hr)) return hr;
  }
  return S_OK;
}
// Get (cached) XSTS for rp. Caller holds lock. out: token copy + uhs.
static HRESULT get_xsts(const char* rp, int force, char** tok, char* uhs, size_t uhsn) {
  HRESULT hr = ensure_chain(); if (FAILED(hr)) return hr;
  Xsts* x = NULL;
  for (int i = 0; i < A.nxsts; i++) if (!strcmp(A.xsts[i].rp, rp)) x = &A.xsts[i];
  if (force || !x || !x->token || x->exp < now_unix() + 120) {
    hr = xbl_xsts(rp);
    if (FAILED(hr)) {   // user token may have been revoked: re-mint once
      clear_user_tokens(); if (FAILED(ensure_chain())) return hr;
      if (FAILED(hr = xbl_xsts(rp))) return hr;
    }
    x = NULL; for (int i = 0; i < A.nxsts; i++) if (!strcmp(A.xsts[i].rp, rp)) x = &A.xsts[i];
    if (!x || !x->token) return E_GU_FAILED_TO_GET_TOKEN;
  }
  *tok = xstrdup(x->token); lcpy(uhs, uhsn, x->uhs[0] ? x->uhs : A.uhs);
  return S_OK;
}

// =============================================================== NSAL (URL -> relying party / signature policy)
typedef struct { char proto[8]; char host[128]; int wildcard; char path[128]; char rp[128]; int sigpol; } Ep;
typedef struct { int version; size_t maxbody; } SigPol;
static Ep* g_eps; static int g_neps; static SigPol g_pols[8]; static int g_npols; static int g_nsal_state; // 0 none, 1 default, 2 +title
static void nsal_add(const char* proto, const char* host, const char* htype, const char* path, const char* rp, int sigpol) {
  if (!host) return;
  g_eps = (Ep*)realloc(g_eps, (g_neps + 1) * sizeof(Ep)); Ep* e = &g_eps[g_neps++]; memset(e, 0, sizeof *e);
  lcpy(e->proto, sizeof e->proto, proto ? proto : "https");
  e->wildcard = (htype && !strcmp(htype, "wildcard")) || host[0] == '*';
  lcpy(e->host, sizeof e->host, e->wildcard && host[0] == '*' ? host + 1 : host);   // wildcard stored as ".suffix"
  for (char* p = e->host; *p; p++) *p = (char)tolower((unsigned char)*p);
  lcpy(e->path, sizeof e->path, path); lcpy(e->rp, sizeof e->rp, rp); e->sigpol = sigpol;
}
static void nsal_parse(J* j, int pol_base) {
  for (J* e = jget(j, "EndPoints") ? jget(j, "EndPoints")->c : NULL; e; e = e->nx) {
    const char* ht = jstr(e, "HostType"); if (ht && strcmp(ht, "fqdn") && strcmp(ht, "wildcard")) continue;
    int sp = jpath(e, "SignaturePolicyIndex") ? jint(e, "SignaturePolicyIndex", -1) : -1;
    nsal_add(jstr(e, "Protocol"), jstr(e, "Host"), ht, jstr(e, "Path"), jstr(e, "RelyingParty"), sp >= 0 ? sp + pol_base : -1);
  }
  for (J* p = jget(j, "SignaturePolicies") ? jget(j, "SignaturePolicies")->c : NULL; p && g_npols < 8; p = p->nx) {
    g_pols[g_npols].version = jint(p, "Version", 1); g_pols[g_npols].maxbody = (size_t)jint(p, "MaxBodyBytes", 8192); g_npols++;
  }
}
static void nsal_builtin(void) {
  // Minecraft-specific services (title NSAL entries) + minimal Xbox Live defaults when the default NSAL can't be fetched.
  if (!g_npols) { g_pols[0].version = 1; g_pols[0].maxbody = 8192; g_npols = 1; }
  nsal_add("https", "*.playfabapi.com", "wildcard", NULL, "http://playfab.xboxlive.com/", -1);
  nsal_add("https", "playfabapi.com", "fqdn", NULL, "http://playfab.xboxlive.com/", -1);
  nsal_add("https", "pocket.realms.minecraft.net", "fqdn", NULL, "https://pocket.realms.minecraft.net/", -1);
  nsal_add("https", "*.realms.minecraft-services.net", "wildcard", NULL, "https://pocket.realms.minecraft.net/", -1);
  nsal_add("https", "*.realms.minecraft.net", "wildcard", NULL, "https://pocket.realms.minecraft.net/", -1);
  nsal_add("https", "multiplayer.minecraft.net", "fqdn", NULL, "https://multiplayer.minecraft.net/", -1);
  nsal_add("https", "*.xboxlive.com", "wildcard", NULL, "http://xboxlive.com", 0);
  nsal_add("https", "xboxlive.com", "fqdn", NULL, "http://xboxlive.com", 0);
}
// Caller holds lock. Loads the default NSAL (anonymous) once, and the title NSAL once a user is signed in.
static void nsal_load(void) {
  if (g_nsal_state == 0) {
    Resp r; HRESULT hr = http_req("GET", "https://title.mgt.xboxlive.com/titles/default/endpoints?type=1", "x-xbl-contract-version: 1\r\nAccept: application/json\r\n", NULL, 0, &r);
    J* j = SUCCEEDED(hr) && r.status == 200 ? jparse(r.body.p) : NULL;
    int before = g_neps;
    if (j) { nsal_parse(j, 0); jfree(j); } else log_err("default NSAL", hr, &r);
    resp_free(&r);
    nsal_builtin();
    xgr_log("XUSER: NSAL: %d endpoints from default NSAL, %d total, %d signature policies", before == g_neps ? 0 : g_neps - before - 8, g_neps, g_npols);
    g_nsal_state = 1;
  }
  if (g_nsal_state == 1 && A.signed_in) {
    g_nsal_state = 2;   // one attempt per process
    char url[160]; snprintf(url, sizeof url, "https://title.mgt.xboxlive.com/titles/%s/endpoints?type=1", A.title_id);
    char* tok = NULL; char uhs[64];
    if (FAILED(get_xsts("http://xboxlive.com", 0, &tok, uhs, sizeof uhs))) return;
    Buf auth = {0}; b_f(&auth, "XBL3.0 x=%s;%s", uhs, tok); free(tok);
    char* sig = xbl_signature("GET", url, auth.p, NULL, 0, 8192);
    Buf h = {0}; b_f(&h, "x-xbl-contract-version: 1\r\nAccept: application/json\r\nAuthorization: %s\r\nSignature: %s\r\n", auth.p, sig ? sig : "");
    free(sig); SecureZeroMemory(auth.p, auth.n); b_free(&auth);
    Resp r; HRESULT hr = http_req("GET", url, h.p, NULL, 0, &r);
    SecureZeroMemory(h.p, h.n); b_free(&h);
    J* j = SUCCEEDED(hr) && r.status == 200 ? jparse(r.body.p) : NULL;
    if (j) {
      // title entries take priority: prepend by parsing into a fresh list, then appending the old one
      Ep* old = g_eps; int nold = g_neps; g_eps = NULL; g_neps = 0;
      int pb = g_npols; nsal_parse(j, pb); jfree(j);
      int ntitle = g_neps;
      g_eps = (Ep*)realloc(g_eps, (g_neps + nold) * sizeof(Ep)); memcpy(g_eps + g_neps, old, nold * sizeof(Ep)); g_neps += nold; free(old);
      xgr_log("XUSER: title NSAL (%s): %d endpoints", A.title_id, ntitle);
      for (int i = 0; i < ntitle; i++) xgr_log("XUSER:   %s %s%s%s -> %s sig=%d", g_eps[i].proto, g_eps[i].wildcard ? "*" : "", g_eps[i].host, g_eps[i].path, g_eps[i].rp[0] ? g_eps[i].rp : "(none)", g_eps[i].sigpol);
    } else log_err("title NSAL", hr, &r);
    resp_free(&r);
  }
}
static const Ep* nsal_match(const char* url) {
  char proto[8] = {0}, host[256] = {0};
  const char* p = strstr(url, "://"); if (!p) return NULL;
  lcpy(proto, sizeof proto, url); if ((size_t)(p - url) < sizeof proto) proto[p - url] = 0;
  for (char* c = proto; *c; c++) *c = (char)tolower((unsigned char)*c);
  if (!strcmp(proto, "wss")) strcpy(proto, "https"); else if (!strcmp(proto, "ws")) strcpy(proto, "http");
  p += 3; size_t hl = strcspn(p, "/?#:"); if (hl >= sizeof host) return NULL;
  for (size_t i = 0; i < hl; i++) host[i] = (char)tolower((unsigned char)p[i]);
  const char* path = p + strcspn(p, "/?#"); if (*path != '/') path = "/";
  const Ep* best = NULL; int bs = -1;
  for (int i = 0; i < g_neps; i++) {
    const Ep* e = &g_eps[i]; int s;
    if (strcmp(e->proto, proto)) continue;
    if (!e->wildcard) { if (strcmp(e->host, host)) continue; s = 100000; }
    else { size_t n = strlen(host), m = strlen(e->host); if (n <= m || strcmp(host + n - m, e->host)) continue; s = 1000 + (int)m; }
    if (e->path[0]) { if (_strnicmp(path, e->path, strlen(e->path))) continue; s += 10000 + (int)strlen(e->path); }
    if (s > bs) { bs = s; best = e; }   // equal score: first wins (title NSAL first)
  }
  return best;
}

// =============================================================== user handle
typedef struct XU { DWORD magic; volatile LONG refs; UINT64 local; } XU;
#define XU_MAGIC 0x52455355UL
static XU g_user = { XU_MAGIC, 1, 1 };
static int user_ok(A64 h) { return h == (A64)&g_user && A.signed_in; }
static XU* user_dup(void) { InterlockedIncrement(&g_user.refs); return &g_user; }
static UINT64 user_xuid(void) { return _strtoui64(A.xid, NULL, 10); }

// change-event registrations
typedef void (*XUserChangeEventCallback)(void* ctx, UINT64 localId, UINT32 evt);
typedef struct Reg { struct Reg* next; UINT64 token; void* queue; void* ctx; XUserChangeEventCallback cb; volatile LONG live; } Reg;
static Reg* g_regs; static SRWLOCK g_regs_lock = SRWLOCK_INIT; static volatile LONG64 g_xu_token = 0x55000000;
typedef struct { Reg* r; UINT32 evt; } Fire;

static impl_fn tqf(const char* name) {
  for (int k = 0; g_taskqueue_impls[k].fn; k++)
    if (g_taskqueue_impls[k].prov1 == TQ_PROV_ && !strcmp(g_taskqueue_impls[k].name, name)) return g_taskqueue_impls[k].fn;
  xgr_log("XUSER: taskqueue impl %s missing", name); return 0;
}
static UINT64 tqc(const char* name, A64 a1, A64 a2, A64 a3, A64 a4, A64 a5) {
  impl_fn f = tqf(name); return f ? f(0, a1, a2, a3, a4, a5, 0) : (UINT64)E_NOTIMPL;
}
static void fire_cb(void* p, unsigned char canceled) {
  Fire* f = (Fire*)p;
  if (!canceled && f->r->live) { xgr_log("XUSER: change event %u -> cb=%p", f->evt, f->r->cb); f->r->cb(f->r->ctx, g_user.local, f->evt); }
  free(f);
}
static void fire_event(UINT32 evt) {
  AcquireSRWLockShared(&g_regs_lock);
  for (Reg* r = g_regs; r; r = r->next) if (r->live) {
    Fire* f = (Fire*)malloc(sizeof *f); f->r = r; f->evt = evt;
    if (FAILED((HRESULT)tqc("XTaskQueueSubmitCallback", (A64)r->queue, 1, (A64)f, (A64)fire_cb, 0))) free(f);
  }
  ReleaseSRWLockShared(&g_regs_lock);
}

// =============================================================== async ops (XAsync provider pattern)
enum { OP_BEGIN_ = 0, OP_DOWORK_ = 1, OP_GETRESULT_ = 2, OP_CANCEL_ = 3, OP_CLEANUP_ = 4 };
typedef struct { void* async; size_t bufferSize; void* buffer; void* context; } PData;
enum { K_ADD = 1, K_TOKEN = 2 };
typedef struct Op {
  volatile LONG refs, cancel; int kind, begun, utf16; UINT32 options;
  void* async;
  char *method, *url; BYTE* body; size_t blen;
  char *tok, *sig; size_t need;
} Op;
static int g_id_add, g_id_tok, g_id_tok16;
static void op_unref(Op* o) {
  if (InterlockedDecrement(&o->refs)) return;
  free(o->method); free(o->url); free(o->body);
  if (o->tok) { SecureZeroMemory(o->tok, strlen(o->tok)); free(o->tok); }
  free(o->sig); free(o);
}
static HRESULT silent_hr(void) { char b[32]; const char* v = envs("XGR_XUSER_SILENT_HR", b, sizeof b, NULL); return v ? (HRESULT)strtoul(v, 0, 16) : (HRESULT)0x89245106L; }
// The real E_GAMEUSER_NO_DEFAULT_USER (0x89245106) makes the game offer interactive sign-in (the "Sign In" button
// then calls XUserAddAsync without the silent flag -> device-code flow). 0x8015DC12 would be a plain failure
// ("Iron Golem") and the button would never escalate. (An earlier startup abort in combase ObjectStublessClient4
// was caused by a patched GameInput service, not by this code.)

// Sign in from the cached refresh token. Caller holds lock.
static HRESULT signin_cached(void) {
  auth_load();
  if (A.signed_in) return S_OK;
  if (!A.msa_refresh) return E_GU_NO_DEFAULT_USER;
  HRESULT hr = ensure_chain();
  if (SUCCEEDED(hr) && !A.xid[0]) { xgr_log("XUSER: signed in but no XUID claim"); hr = E_GU_FAILED_TO_GET_TOKEN; }
  if (SUCCEEDED(hr)) { InterlockedExchange(&A.signed_in, 1); auth_save(); xgr_log("XUSER: signed in as %s (xuid %s)", A.gtg, A.xid); }
  return hr;
}
static HRESULT do_add(Op* o) {
  int silent = (o->options & 1) != 0;
  if (A.signed_in) return S_OK;
  if (silent && A.interactive) return silent_hr();   // an interactive sign-in is running: don't block
  auth_lock(); HRESULT hr = signin_cached(); auth_unlock();
  if (SUCCEEDED(hr)) return hr;
  if (silent) { xgr_log("XUSER: silent add -> no cached user (%08lx)", (unsigned long)hr); return silent_hr(); }
  if (InterlockedCompareExchange(&A.interactive, 1, 0)) {
    xgr_log("XUSER: interactive sign-in already running; waiting for it");
    while (A.interactive && !o->cancel) Sleep(200);
    return A.signed_in ? S_OK : E_ABORT;
  }
  xgr_log("XUSER: starting interactive Microsoft account sign-in (device code, client %s)", A.client_id);
  hr = msa_device_flow(&o->cancel, NULL, 0);
  if (hr == S_OK) {
    auth_lock();
    auth_save();                          // persist refresh token + device key immediately
    A.user_exp = 0; hr = signin_cached();
    auth_unlock();
    if (FAILED(hr)) xgr_log("XUSER: Xbox Live token chain failed %08lx (account may need an Xbox profile: sign in once at https://www.xbox.com)", (unsigned long)hr);
  } else if (hr == S_FALSE) hr = E_ABORT;
  char t[256];
  if (SUCCEEDED(hr)) snprintf(t, sizeof t, "Signed in as %s (xuid %s)\r\n", A.gtg, A.xid);
  else snprintf(t, sizeof t, "Sign-in did not complete (hr=%08lx)\r\n", (unsigned long)hr);
  signin_file(t);
  InterlockedExchange(&A.interactive, 0);
  if (SUCCEEDED(hr)) { int ev = envi("XGR_XUSER_SIGNIN_EVENT", -1); if (ev >= 0) fire_event((UINT32)ev); }
  return FAILED(hr) ? (hr == E_ABORT ? E_ABORT : hr) : S_OK;
}
static HRESULT do_token(Op* o) {
  if (!A.signed_in) return E_GU_SIGNED_OUT;
  auth_lock();
  nsal_load();
  const Ep* e = nsal_match(o->url);
  HRESULT hr = S_OK; char* tok = NULL; char uhs[64] = {0};
  if (!e || !e->rp[0]) {
    xgr_log("XUSER: token for %s %s: no relying party -> empty token", o->method, o->url);
    o->tok = xstrdup(""); o->sig = xstrdup("");
  } else if (SUCCEEDED(hr = get_xsts(e->rp, o->options & 1, &tok, uhs, sizeof uhs))) {
    Buf b = {0}; b_f(&b, "XBL3.0 x=%s;%s", uhs, tok); SecureZeroMemory(tok, strlen(tok)); free(tok);
    o->tok = b.p;
    size_t maxb = 8192; if (e->sigpol >= 0 && e->sigpol < g_npols) maxb = g_pols[e->sigpol].maxbody;
    o->sig = e->sigpol >= 0 ? xbl_signature(o->method, o->url, o->tok, o->body, o->blen, maxb) : xstrdup("");
    if (!o->sig) o->sig = xstrdup("");
    xgr_log("XUSER: token for %s %s -> rp=%s token=%u bytes sig=%s", o->method, o->url, e->rp, (unsigned)strlen(o->tok), o->sig[0] ? "yes" : "none");
  } else xgr_log("XUSER: token for %s %s (rp %s) failed %08lx", o->method, o->url, e->rp, (unsigned long)hr);
  auth_unlock();
  if (SUCCEEDED(hr)) {
    size_t tl = strlen(o->tok) + 1, sl = strlen(o->sig) + 1;
    o->need = o->utf16 ? 32 + (tl + sl) * sizeof(WCHAR) : 32 + tl + sl;
  }
  return FAILED(hr) ? hr : S_OK;
}
static DWORD WINAPI op_thread(LPVOID p) {
  Op* o = (Op*)p; HRESULT hr; size_t need = 0;
  if (o->kind == K_ADD) { hr = do_add(o); if (SUCCEEDED(hr)) need = sizeof(void*); }
  else { hr = do_token(o); if (SUCCEEDED(hr)) need = o->need; }
  if (o->cancel && FAILED(hr)) hr = E_ABORT;
  tqc("XAsyncComplete", (A64)o->async, (A64)(UINT32)hr, need, 0, 0);
  op_unref(o);
  return 0;
}
static HRESULT op_provider(int op, const PData* d) {
  Op* o = (Op*)d->context;
  switch (op) {
  case OP_BEGIN_: {
    o->begun = 1; o->async = d->async; InterlockedIncrement(&o->refs);
    HANDLE t = CreateThread(NULL, 0, op_thread, o, 0, NULL);
    if (!t) { InterlockedDecrement(&o->refs); return HRESULT_FROM_WIN32(GetLastError()); }
    CloseHandle(t); return S_OK; }
  case OP_GETRESULT_:
    if (o->kind == K_ADD) {
      if (d->bufferSize < sizeof(void*)) return E_NOT_SUFFICIENT_BUFFER__;
      *(void**)d->buffer = user_dup(); return S_OK;
    } else {
      if (d->bufferSize < o->need) return E_NOT_SUFFICIENT_BUFFER__;
      BYTE* b = (BYTE*)d->buffer; size_t tl = strlen(o->tok) + 1, sl = strlen(o->sig) + 1;
      size_t* sz = (size_t*)b; void** ptr = (void**)(b + 16);
      if (!o->utf16) {
        char* t = (char*)(b + 32); char* s = t + tl;
        memcpy(t, o->tok, tl); memcpy(s, o->sig, sl);
        sz[0] = tl; sz[1] = sl; ptr[0] = t; ptr[1] = s;
      } else {
        WCHAR* t = (WCHAR*)(b + 32); WCHAR* s = t + tl;   // token/signature are ASCII
        for (size_t i = 0; i < tl; i++) t[i] = (WCHAR)(unsigned char)o->tok[i];
        for (size_t i = 0; i < sl; i++) s[i] = (WCHAR)(unsigned char)o->sig[i];
        sz[0] = tl; sz[1] = sl; ptr[0] = t; ptr[1] = s;
      }
      return S_OK;
    }
  case OP_CANCEL_: InterlockedExchange(&o->cancel, 1); return S_OK;
  case OP_CLEANUP_: op_unref(o); return S_OK;
  default: return S_OK;
  }
}
static HRESULT op_start(Op* o, A64 async, void* identity, const char* name) {
  o->refs = 1;
  HRESULT hr = (HRESULT)tqc("XAsyncBegin", async, (A64)o, (A64)identity, (A64)name, (A64)op_provider);
  if (FAILED(hr) && !o->begun) op_unref(o);
  return hr;
}

// =============================================================== XUser slots
static int xuser_off(void) { static int v = -1; if (v < 0) { char b[16]; const char* s = envs("XGR_XUSER", b, sizeof b, ""); v = !_stricmp(s, "off"); } return v; }
static UINT64 sys_fallback(Obj* o, int slot, A64 a1, A64 a2, A64 a3, A64 a4, A64 a5, A64 a6) {
  for (int k = 0; g_system_impls[k].fn; k++) if (g_system_impls[k].prov1 == XU_PROV && g_system_impls[k].slot == slot) return g_system_impls[k].fn(o, a1, a2, a3, a4, a5, a6);
  return (UINT64)E_NOTIMPL;
}
#define OFFCHK(slot) if (xuser_off()) return sys_fallback(o, slot, a1, a2, a3, a4, a5, a6)

// s3: XUserDuplicateHandle(user, dup*)  | iid cef4fac0 s3: XUserGetGamertag(user, component, size, char*, used*)
IMPL(Slot3_) {
  if (o->iid.Data1 == IID_GAMERTAG) {
    if (!user_ok(a1)) return (UINT64)E_INVALIDARG;
    const char* g; char tmp[160];
    switch ((UINT32)a2) {
    case 0: g = A.gtg; break;
    case 1: g = A.mgt[0] ? A.mgt : A.gtg; break;
    case 2: g = A.mgs; break;
    case 3: if (A.umg[0]) g = A.umg; else if (A.mgt[0] && A.mgs[0]) { snprintf(tmp, sizeof tmp, "%s#%s", A.mgt, A.mgs); g = tmp; } else g = A.mgt[0] ? A.mgt : A.gtg; break;
    default: return (UINT64)E_INVALIDARG;
    }
    size_t n = strlen(g) + 1;
    if (a5) *(size_t*)a5 = n;
    if (!a4 || a3 < n) return (UINT64)E_NOT_SUFFICIENT_BUFFER__;
    memcpy((char*)a4, g, n); return S_OK;
  }
  OFFCHK(3);
  if (!a2) return (UINT64)E_INVALIDARG;
  if (a1 != (A64)&g_user) { *(void**)a2 = NULL; return (UINT64)E_INVALIDARG; }
  *(void**)a2 = user_dup(); return S_OK;
}
IMPL(CloseHandle_) { OFFCHK(4); if (a1 == (A64)&g_user) InterlockedDecrement(&g_user.refs); return 0; }
IMPL(Compare_) {
  OFFCHK(5);
  UINT64 l1 = a1 == (A64)&g_user ? g_user.local : 0, l2 = a2 == (A64)&g_user ? g_user.local : 0;
  return (UINT64)(UINT32)(l1 == l2 ? 0 : (l1 < l2 ? -1 : 1));
}
IMPL(GetMaxUsers_) { if (!a1) return (UINT64)E_INVALIDARG; *(UINT32*)a1 = 1; return S_OK; }
IMPL(AddAsync_) {
  OFFCHK(7);
  if (!a2) return (UINT64)E_INVALIDARG;
  Op* op = (Op*)calloc(1, sizeof *op); op->kind = K_ADD; op->options = (UINT32)a1;
  xgr_log("XUSER: XUserAddAsync(options=%llx) signed_in=%ld cached=%s", a1, A.signed_in, A.msa_refresh ? "yes" : (A.loaded ? "no" : "?"));
  return (UINT64)(LONG)op_start(op, a2, &g_id_add, "XUserAddAsync");
}
IMPL(AddResult_) {
  OFFCHK(8);
  void* h = NULL;
  HRESULT hr = (HRESULT)tqc("XAsyncGetResult", a1, (A64)&g_id_add, sizeof h, (A64)&h, 0);
  if (a2) *(void**)a2 = SUCCEEDED(hr) ? h : NULL;
  xgr_log("XUSER: XUserAddResult -> %08lx handle=%p", (unsigned long)hr, SUCCEEDED(hr) ? h : NULL);
  return (UINT64)(LONG)hr;
}
IMPL(GetLocalId_) { if (!a2) return (UINT64)E_INVALIDARG; if (a1 != (A64)&g_user) return (UINT64)E_INVALIDARG; *(UINT64*)a2 = g_user.local; return S_OK; }
IMPL(FindUserByLocalId_) {
  if (!a2) return (UINT64)E_INVALIDARG;
  if (A.signed_in && a1 == g_user.local) { *(void**)a2 = user_dup(); return S_OK; }
  *(void**)a2 = NULL; return (UINT64)E_GU_USER_NOT_FOUND;
}
IMPL(GetId_) { if (!a2 || !user_ok(a1)) return (UINT64)E_INVALIDARG; *(UINT64*)a2 = user_xuid(); return S_OK; }
IMPL(FindUserById_) {
  if (!a2) return (UINT64)E_INVALIDARG;
  if (A.signed_in && a1 == user_xuid()) { *(void**)a2 = user_dup(); return S_OK; }
  *(void**)a2 = NULL; return (UINT64)E_GU_USER_NOT_FOUND;
}
IMPL(GetIsGuest_) { if (!a2) return (UINT64)E_INVALIDARG; *(BYTE*)a2 = 0; return S_OK; }
IMPL(GetState_) { if (!a2) return (UINT64)E_INVALIDARG; *(UINT32*)a2 = user_ok(a1) ? 0 : 2; return S_OK; }
IMPL(GetAgeGroup_) {
  if (!a2) return (UINT64)E_INVALIDARG;
  UINT32 g = 3;
  if (!_stricmp(A.agg, "Child")) g = 1; else if (!_stricmp(A.agg, "Teen")) g = 2;
  *(UINT32*)a2 = g; return S_OK;
}
// XUserCheckPrivilege(user, options, privilege, BOOLEAN* has, reason*)
IMPL(CheckPrivilege_) {
  if (!a4) return (UINT64)E_INVALIDARG;
  int has = 1;
  if (A.prv && *A.prv && !envi("XGR_XUSER_PRIV_ALL", 0)) {
    char id[16]; snprintf(id, sizeof id, "%u", (UINT32)a3); has = 0;
    for (const char* p = A.prv; *p;) {
      while (*p == ' ') { p++; }
      size_t n = strcspn(p, " ");
      if (n == strlen(id) && !strncmp(p, id, n)) { has = 1; break; }
      p += n;
    }
  }
  *(BYTE*)a4 = (BYTE)has; if (a5) *(UINT32*)a5 = has ? 0 : 2;
  static UINT32 logged[8]; static int nl;
  int seen = 0; for (int i = 0; i < nl; i++) if (logged[i] == (UINT32)a3) seen = 1;
  if (!seen && nl < 8) { logged[nl++] = (UINT32)a3; xgr_log("XUSER: CheckPrivilege(%u) -> %d", (UINT32)a3, has); }
  return S_OK;
}
// XUserGetTokenAndSignature[Utf16]Async(user, options, method, url, headerCount, headers, bodySize, body, async)
static UINT64 token_async(A64 a1, A64 a2, const char* method, const char* url, A64 a7, A64 a8, A64 a9, int utf16) {
  if (!a9 || !method || !url) return (UINT64)E_INVALIDARG;
  if (!user_ok(a1)) return (UINT64)E_GU_SIGNED_OUT;
  Op* op = (Op*)calloc(1, sizeof *op); op->kind = K_TOKEN; op->options = (UINT32)a2; op->utf16 = utf16;
  op->method = xstrdup(method); op->url = xstrdup(url);
  if (a7 && a8) { op->body = (BYTE*)malloc(a7); memcpy(op->body, (void*)a8, a7); op->blen = a7; }
  return (UINT64)(LONG)op_start(op, a9, utf16 ? &g_id_tok16 : &g_id_tok, "XUserGetTokenAndSignatureAsync");
}
IMPL9(TokenAsync_) { return token_async(a1, a2, (const char*)a3, (const char*)a4, a7, a8, a9, 0); }
IMPL9(TokenAsync16_) {
  char* m = narrow((const WCHAR*)a3); char* u = narrow((const WCHAR*)a4);
  UINT64 r = token_async(a1, a2, m, u, a7, a8, a9, 1); free(m); free(u); return r;
}
IMPL(TokenResultSize_) { return tqc("XAsyncGetResultSize", a1, a2, 0, 0, 0); }
static UINT64 token_result(A64 a1, A64 a2, A64 a3, A64 a4, A64 a5, void* id) {
  HRESULT hr = (HRESULT)tqc("XAsyncGetResult", a1, (A64)id, a2, a3, a5);
  if (a4) *(void**)a4 = SUCCEEDED(hr) ? (void*)a3 : NULL;
  return (UINT64)(LONG)hr;
}
IMPL(TokenResult_) { return token_result(a1, a2, a3, a4, a5, &g_id_tok); }
IMPL(TokenResult16_) { return token_result(a1, a2, a3, a4, a5, &g_id_tok16); }
IMPL(RegisterChange_) {
  OFFCHK(33);
  if (!a3 || !a4) return (UINT64)E_INVALIDARG;
  Reg* r = (Reg*)calloc(1, sizeof *r); r->token = InterlockedIncrement64(&g_xu_token); r->queue = (void*)a1; r->ctx = (void*)a2; r->cb = (XUserChangeEventCallback)a3; r->live = 1;
  AcquireSRWLockExclusive(&g_regs_lock); r->next = g_regs; g_regs = r; ReleaseSRWLockExclusive(&g_regs_lock);
  *(UINT64*)a4 = r->token; return S_OK;
}
IMPL(UnregisterChange_) {
  AcquireSRWLockShared(&g_regs_lock);
  for (Reg* r = g_regs; r; r = r->next) if (r->token == a1) InterlockedExchange(&r->live, 0);
  ReleaseSRWLockShared(&g_regs_lock);
  return 1;
}
IMPL(ret_notimpl_) { return (UINT64)E_NOTIMPL; }
IMPL(SignOutDeferral_) { if (a1) *(void**)a1 = NULL; return (UINT64)E_GU_DEFERRAL_NOT_AVAILABLE; }
IMPL(ret_void_) { return 0; }
IMPL(IsStoreUser_) { return 1; }
// XUserDevice (7d824997): s3 XUserFindForDevice(deviceId*, handle*), s4 register assoc, s5 unregister
IMPL(FindForDevice_) {
  if (!a2) return (UINT64)E_INVALIDARG;
  if (A.signed_in) { *(void**)a2 = user_dup(); return S_OK; }
  return (UINT64)E_NOTIMPL;   // pre-existing behaviour when nobody is signed in (0x89245104 here led to a combase abort)
}
static volatile LONG64 g_dev_token = 0x56000000;
IMPL(RegDevice_) { if (a4) *(UINT64*)a4 = InterlockedIncrement64(&g_dev_token); return S_OK; }
IMPL(ret_true_) { return 1; }

const Impl g_xuser_impls[] = {
  {XU_PROV, 3, Slot3_, "XUserDuplicateHandle/XUserGetGamertag"},
  {XU_PROV, 4, CloseHandle_, "XUserCloseHandle"},
  {XU_PROV, 5, Compare_, "XUserCompare"},
  {XU_PROV, 6, GetMaxUsers_, "XUserGetMaxUsers"},
  {XU_PROV, 7, AddAsync_, "XUserAddAsync"},
  {XU_PROV, 8, AddResult_, "XUserAddResult"},
  {XU_PROV, 9, GetLocalId_, "XUserGetLocalId"},
  {XU_PROV, 10, FindUserByLocalId_, "XUserFindUserByLocalId"},
  {XU_PROV, 11, GetId_, "XUserGetId"},
  {XU_PROV, 12, FindUserById_, "XUserFindUserById"},
  {XU_PROV, 13, GetIsGuest_, "XUserGetIsGuest"},
  {XU_PROV, 14, GetState_, "XUserGetState"},
  {XU_PROV, 16, ret_notimpl_, "XUserGetGamerPictureAsync"},
  {XU_PROV, 19, GetAgeGroup_, "XUserGetAgeGroup"},
  {XU_PROV, 20, CheckPrivilege_, "XUserCheckPrivilege"},
  {XU_PROV, 21, ret_notimpl_, "XUserResolvePrivilegeWithUiAsync"},
  {XU_PROV, 23, (impl_fn)TokenAsync_, "XUserGetTokenAndSignatureAsync"},
  {XU_PROV, 24, TokenResultSize_, "XUserGetTokenAndSignatureResultSize"},
  {XU_PROV, 25, TokenResult_, "XUserGetTokenAndSignatureResult"},
  {XU_PROV, 26, (impl_fn)TokenAsync16_, "XUserGetTokenAndSignatureUtf16Async"},
  {XU_PROV, 27, TokenResultSize_, "XUserGetTokenAndSignatureUtf16ResultSize"},
  {XU_PROV, 28, TokenResult16_, "XUserGetTokenAndSignatureUtf16Result"},
  {XU_PROV, 29, ret_notimpl_, "XUserResolveIssueWithUiAsync"},
  {XU_PROV, 31, ret_notimpl_, "XUserResolveIssueWithUiUtf16Async"},
  {XU_PROV, 33, RegisterChange_, "XUserRegisterForChangeEvent"},
  {XU_PROV, 34, UnregisterChange_, "XUserUnregisterForChangeEvent"},
  {XU_PROV, 35, SignOutDeferral_, "XUserGetSignOutDeferral"},
  {XU_PROV, 36, ret_void_, "XUserCloseSignOutDeferralHandle"},
  {XU_PROV, 42, IsStoreUser_, "XUserIsStoreUser"},
  {XUD_PROV, 3, FindForDevice_, "XUserFindForDevice"},
  {XUD_PROV, 4, RegDevice_, "XUserRegisterForDeviceAssociationChanged"},
  {XUD_PROV, 5, ret_true_, "XUserUnregisterForDeviceAssociationChanged"},
  {0, 0, 0, 0}
};
