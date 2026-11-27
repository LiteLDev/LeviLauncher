// Shared header for the xgameruntime.dll shim.
// The GDK static-lib thunks in the game call QueryApiImpl(provider, iid, &obj) and then
// invoke obj->vtbl[slot](obj, args...). Slots 0..2 are QI/AddRef/Release.
// Each module provides an Impl table keyed by (provider GUID Data1, slot).
// impl_fn receives `this` then up to 6 integer/pointer args (extra args are harmless).
// Struct-returning methods (MSVC ABI) get the hidden return pointer as a1 and must return it.
#pragma once
#include <windows.h>
#include <stdio.h>
#include <string.h>
typedef struct Obj { void** vtbl; GUID prov, iid; LONG ref; } Obj;
typedef UINT64 (*impl_fn)(Obj*, UINT64, UINT64, UINT64, UINT64, UINT64, UINT64);
// slots are invoked with 9 args (a7..a9 = stack args 7..9); impls needing them declare 9 params and cast to impl_fn
typedef UINT64 (*impl9_fn)(Obj*, UINT64, UINT64, UINT64, UINT64, UINT64, UINT64, UINT64, UINT64, UINT64);
typedef struct { unsigned long prov1; int slot; impl_fn fn; const char* name; } Impl;
void xgr_log(const char* fmt, ...);
void xgr_start_mods(void);      // extra_mods.c: load <exe dir>\mods\*.dll after the window appears
extern const Impl g_system_impls[];     // system.c
extern const Impl g_taskqueue_impls[];  // taskqueue.c (XTaskQueue / XAsync, provider 073b7dcb)
extern const Impl g_networking_impls[]; // extra_networking.c (XNetworking 37e56907; listed first in g_tables)
extern const Impl g_xuser_impls[];     // extra_xuser.c (XUser 01acd177 / 7d824997; before g_system_impls)
