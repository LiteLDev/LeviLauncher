#include "xgr.h"
// Mod loader: loads every *.dll in "<directory of the game exe>\mods" (sorted by name) once the game window
// exists, similar to what an external injector would do. Started from the first InitializeApiImpl* call.
//   XGR_MODS=0             disable
//   XGR_MODS_DELAY_MS=<n>  extra delay after the window appears (default 2000)
// Early mods: every *.dll in "<game dir>\mods_early" is loaded synchronously inside that first InitializeApiImpl* call,
// on the game's thread, before the game creates its window or renderer (mods that must hook the renderer's creation,
// e.g. HeadlessCore.dll). XGR_MODS_EARLY=1 loads "mods" that way too (no window wait, no delay).
#include <stdlib.h>
#include <wchar.h>

static int wcmp(const void* a, const void* b) { return _wcsicmp((const WCHAR*)a, (const WCHAR*)b); }

// sorted *.dll names of `dir` into names (at most 64); returns the count, -1 when the folder has none
static int list_mods(const WCHAR* dir, WCHAR (*names)[MAX_PATH]) {
  WCHAR pat[MAX_PATH];
  swprintf(pat, MAX_PATH, L"%ls\\*.dll", dir);
  int n = 0; WIN32_FIND_DATAW fd; HANDLE f = FindFirstFileW(pat, &fd);
  if (f == INVALID_HANDLE_VALUE) return -1;
  do { if (!(fd.dwFileAttributes & FILE_ATTRIBUTE_DIRECTORY) && n < 64) wcscpy(names[n++], fd.cFileName); } while (FindNextFileW(f, &fd));
  FindClose(f);
  qsort(names, n, sizeof names[0], wcmp);
  return n;
}

static void load_mods(const WCHAR* dir, WCHAR (*names)[MAX_PATH], int n) {
  WCHAR path[MAX_PATH];
  SetDllDirectoryW(dir);  // let mods resolve their own side-by-side DLLs from the mods folder
  for (int i = 0; i < n; i++) {
    swprintf(path, MAX_PATH, L"%ls\\%ls", dir, names[i]);
    HMODULE h = LoadLibraryW(path);
    if (h) xgr_log("MODS: loaded %ls at %p", names[i], h);
    else xgr_log("MODS: FAILED to load %ls (error %lu)", names[i], GetLastError());
  }
  SetDllDirectoryW(NULL);
}

static void game_subdir(WCHAR* dir, const WCHAR* name) {
  GetModuleFileNameW(NULL, dir, MAX_PATH);
  WCHAR* slash = wcsrchr(dir, L'\\'); if (slash) *slash = 0;
  wcscat(dir, L"\\"); wcscat(dir, name);
}

static int env_on(const char* name) { char e[8]; return GetEnvironmentVariableA(name, e, sizeof e) && e[0] == '1'; }

static DWORD WINAPI mods_thread(void* unused) {
  static WCHAR names[64][MAX_PATH];
  WCHAR dir[MAX_PATH];
  char env[32];
  DWORD delay = GetEnvironmentVariableA("XGR_MODS_DELAY_MS", env, sizeof env) ? strtoul(env, 0, 10) : 2000;
  game_subdir(dir, L"mods");
  int n = list_mods(dir, names);
  if (n < 0) { xgr_log("MODS: no mods in %ls", dir); return 0; }
  xgr_log("MODS: %d mod(s) in %ls, waiting for the game window", n, dir);
  for (int i = 0; i < 1200 && !FindWindowW(L"Bedrock", NULL); i++) Sleep(100);
  Sleep(delay);
  load_mods(dir, names, n);
  return 0;
}

void xgr_start_mods(void) {
  static LONG started;
  static WCHAR names[64][MAX_PATH];
  WCHAR dir[MAX_PATH];
  char env[8];
  if (InterlockedExchange(&started, 1)) return;
  if (GetEnvironmentVariableA("XGR_MODS", env, sizeof env) && env[0] == '0') { xgr_log("MODS: disabled"); return; }
  // early mods: synchronously, right here on the game's thread
  game_subdir(dir, L"mods_early");
  int n = list_mods(dir, names);
  if (n > 0) { xgr_log("MODS: %d early mod(s) in %ls, loading now (thread %04lx)", n, dir, GetCurrentThreadId()); load_mods(dir, names, n); }
  if (env_on("XGR_MODS_EARLY")) {
    game_subdir(dir, L"mods");
    n = list_mods(dir, names);
    if (n > 0) { xgr_log("MODS: XGR_MODS_EARLY=1: %d mod(s) in %ls, loading now", n, dir); load_mods(dir, names, n); }
    else xgr_log("MODS: no mods in %ls", dir);
    return;
  }
  HANDLE t = CreateThread(NULL, 0, mods_thread, NULL, 0, NULL);
  if (t) CloseHandle(t);
}
