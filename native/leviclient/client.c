/* SPDX-License-Identifier: GPL-3.0-only */
#define WIN32_LEAN_AND_MEAN
#include <windows.h>
#include <winhttp.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include "protocol.h"

typedef int (WINAPI *MessageBoxFn)(HWND, LPCWSTR, LPCWSTR, UINT);
static MessageBoxFn original_message_box;
static char endpoint[256], token[96];
typedef void *(__cdecl *OpenFileFn)(const char *, const char *);
typedef int (__cdecl *PutStringFn)(const char *, void *);
typedef int (__cdecl *CloseFileFn)(void *);
static OpenFileFn original_open;
static PutStringFn original_put;
static CloseFileFn original_close;
static PVOID volatile sign_in_file;
static int result_events;

static int post_json(const char *suffix, const char *body) {
    const char *prefix = "http://127.0.0.1:";
    if (strncmp(endpoint, prefix, strlen(prefix))) return 0;
    char *path;
    unsigned long port = strtoul(endpoint + strlen(prefix), &path, 10);
    if (port == 0 || port > 65535 || strncmp(path, "/v1/game-login/", 15)) return 0;
    char full_path[160];
    int path_length = snprintf(full_path, sizeof full_path, "%s%s", path, suffix);
    if (path_length < 0 || path_length >= (int)sizeof full_path) return 0;
    WCHAR request_path[160], headers[192];
    if (!MultiByteToWideChar(CP_UTF8, MB_ERR_INVALID_CHARS, full_path, -1, request_path, 160)) return 0;
    WCHAR wide_token[96];
    if (!MultiByteToWideChar(CP_UTF8, MB_ERR_INVALID_CHARS, token, -1, wide_token, 96)) return 0;
    _snwprintf(headers, 192, L"Authorization: Bearer %ls\r\nContent-Type: application/json\r\n", wide_token);
    headers[191] = 0;
    DWORD length = (DWORD)strlen(body);
    int accepted = 0;
    HINTERNET session = WinHttpOpen(L"LeviClient/0.1", WINHTTP_ACCESS_TYPE_NO_PROXY, NULL, NULL, 0);
    if (!session) return 0;
    WinHttpSetTimeouts(session, 1000, 1000, 1000, 1500);
    HINTERNET connection = WinHttpConnect(session, L"127.0.0.1", (INTERNET_PORT)port, 0);
    HINTERNET request = connection ? WinHttpOpenRequest(connection, L"POST", request_path, NULL, NULL, NULL, 0) : NULL;
    if (request && WinHttpSendRequest(request, headers, (DWORD)wcslen(headers), (LPVOID)body, length, length, 0) && WinHttpReceiveResponse(request, NULL)) {
        DWORD status = 0, size = sizeof status;
        if (WinHttpQueryHeaders(request, WINHTTP_QUERY_STATUS_CODE | WINHTTP_QUERY_FLAG_NUMBER, NULL, &status, &size, NULL))
            accepted = status == 200;
    }
    if (request) WinHttpCloseHandle(request);
    if (connection) WinHttpCloseHandle(connection);
    WinHttpCloseHandle(session);
    return accepted;
}

static int forward_code(const char *code) {
    char body[96];
    snprintf(body, sizeof body, "{\"code\":\"%s\",\"expiresIn\":900,\"resultEvents\":%s}", code, result_events ? "true" : "false");
    return post_json("", body);
}

static void *__cdecl bridge_open(const char *path, const char *mode) {
    void *file = original_open(path, mode);
    if (file && path && !_stricmp(path, "C:\\xgr_signin.txt")) InterlockedExchangePointer(&sign_in_file, file);
    return file;
}

static int __cdecl bridge_put(const char *text, void *file) {
    int result = original_put(text, file);
    if (file == sign_in_file && text) {
        if (!strncmp(text, "Signed in as ", 13)) (void)post_json("/result", "{\"phase\":\"success\"}");
        else {
            char error[9];
            if (sscanf(text, "Sign-in did not complete (hr=%8[0123456789abcdefABCDEF])", error) == 1 && strlen(error) == 8) {
                char body[80];
                snprintf(body, sizeof body, "{\"phase\":\"failed\",\"errorCode\":\"%s\"}", error);
                (void)post_json("/result", body);
            }
        }
    }
    return result;
}

static int __cdecl bridge_close(void *file) {
    InterlockedCompareExchangePointer(&sign_in_file, NULL, file);
    return original_close(file);
}

static int WINAPI bridge_message_box(HWND owner, LPCWSTR text, LPCWSTR title, UINT flags) {
    char code[9];
    if (title && !lstrcmpW(title, L"Minecraft - Microsoft account sign-in") &&
        levi_device_code(text, code) && forward_code(code)) return IDOK;
    return original_message_box(owner, text, title, flags);
}

static int install_bridge(HMODULE runtime) {
    BYTE *base = (BYTE *)runtime;
    IMAGE_DOS_HEADER *dos = (IMAGE_DOS_HEADER *)base;
    if (dos->e_magic != IMAGE_DOS_SIGNATURE) return 0;
    IMAGE_NT_HEADERS64 *nt = (IMAGE_NT_HEADERS64 *)(base + dos->e_lfanew);
    if (nt->Signature != IMAGE_NT_SIGNATURE || nt->FileHeader.Machine != IMAGE_FILE_MACHINE_AMD64) return 0;
    DWORD rva = nt->OptionalHeader.DataDirectory[IMAGE_DIRECTORY_ENTRY_IMPORT].VirtualAddress;
    if (!rva) return 0;
    for (IMAGE_IMPORT_DESCRIPTOR *desc = (IMAGE_IMPORT_DESCRIPTOR *)(base + rva); desc->Name; ++desc) {
        if (!desc->OriginalFirstThunk) continue;
        IMAGE_THUNK_DATA64 *lookup = (IMAGE_THUNK_DATA64 *)(base + desc->OriginalFirstThunk);
        IMAGE_THUNK_DATA64 *iat = (IMAGE_THUNK_DATA64 *)(base + desc->FirstThunk);
        for (; lookup->u1.AddressOfData; ++lookup, ++iat) {
            if (IMAGE_SNAP_BY_ORDINAL64(lookup->u1.Ordinal)) continue;
            IMAGE_IMPORT_BY_NAME *name = (IMAGE_IMPORT_BY_NAME *)(base + lookup->u1.AddressOfData);
            const char *symbol = (char *)name->Name;
            PVOID replacement = NULL;
            PVOID *original = NULL;
            if (!strcmp(symbol, "MessageBoxW")) { replacement = (PVOID)&bridge_message_box; original = (PVOID *)&original_message_box; }
            else if (!strcmp(symbol, "fopen")) { replacement = (PVOID)&bridge_open; original = (PVOID *)&original_open; }
            else if (!strcmp(symbol, "fputs")) { replacement = (PVOID)&bridge_put; original = (PVOID *)&original_put; }
            else if (!strcmp(symbol, "fclose")) { replacement = (PVOID)&bridge_close; original = (PVOID *)&original_close; }
            if (!replacement || (PVOID)(ULONG_PTR)iat->u1.Function == replacement) continue;
            HMODULE pinned;
            if (!GetModuleHandleExW(GET_MODULE_HANDLE_EX_FLAG_FROM_ADDRESS | GET_MODULE_HANDLE_EX_FLAG_PIN,
                                   (LPCWSTR)&bridge_message_box, &pinned)) return 0;
            DWORD protection;
            if (!VirtualProtect(&iat->u1.Function, sizeof(void *), PAGE_READWRITE, &protection)) return 0;
            *original = (PVOID)(ULONG_PTR)iat->u1.Function;
            InterlockedExchangePointer((PVOID volatile *)&iat->u1.Function, replacement);
            VirtualProtect(&iat->u1.Function, sizeof(void *), protection, &protection);
        }
    }
    result_events = original_open && original_put && original_close;
    if (original_message_box) OutputDebugStringA("LeviClient: game account bridge ready\n");
    return original_message_box != NULL;
}

static DWORD WINAPI attach_runtime(void *unused) {
    (void)unused;
    for (int i = 0; i < 600; ++i) {
        HMODULE runtime = GetModuleHandleW(L"xgameruntime.dll");
        if (runtime && install_bridge(runtime)) return 0;
        Sleep(100);
    }
    return 0;
}

BOOL WINAPI DllMain(HINSTANCE module, DWORD reason, LPVOID reserved) {
    (void)reserved;
    if (reason != DLL_PROCESS_ATTACH) return TRUE;
    DisableThreadLibraryCalls(module);
    DWORD a = GetEnvironmentVariableA("LEVI_GAME_AUTH_URL", endpoint, sizeof endpoint);
    DWORD b = GetEnvironmentVariableA("LEVI_GAME_AUTH_TOKEN", token, sizeof token);
    /* Inert on ordinary Windows launches, or when no launcher bridge exists. */
    if (!a || a >= sizeof endpoint || !b || b >= sizeof token) return TRUE;
    HANDLE thread = CreateThread(NULL, 0, attach_runtime, NULL, 0, NULL);
    if (thread) CloseHandle(thread);
    return TRUE;
}
