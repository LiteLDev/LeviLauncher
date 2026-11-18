#ifndef LEVI_CLIENT_PROTOCOL_H
#define LEVI_CLIENT_PROTOCOL_H
#include <stddef.h>
#include <wchar.h>

/* Only the known XGR device-code prompt is forwarded, never arbitrary dialog text. */
static int levi_device_code(const wchar_t *text, char code[9]) {
    const wchar_t *start = text ? wcsstr(text, L"enter the code") : NULL;
    if (!start) return 0;
    start += wcslen(L"enter the code");
    while (*start == L' ' || *start == L'\r' || *start == L'\n' || *start == L':') ++start;
    for (int i = 0; i < 8; ++i) {
        wchar_t c = start[i];
        if (!((c >= L'A' && c <= L'Z') || (c >= L'0' && c <= L'9'))) return 0;
        code[i] = (char)c;
    }
    if (start[8] && start[8] != L'\r' && start[8] != L'\n' && start[8] != L' ') return 0;
    code[8] = 0;
    return 1;
}
#endif
