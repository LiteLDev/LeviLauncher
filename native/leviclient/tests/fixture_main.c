#include <windows.h>
#include <stdio.h>
int main(void) {
    HMODULE runtime = LoadLibraryW(L"xgameruntime.dll");
    HMODULE client = LoadLibraryW(L"LeviClient.dll");
    Sleep(500);
    int prompt = -1, other = -1;
    if (runtime && client) {
        int (*show)(void) = (int (*)(void))GetProcAddress(runtime, "fixture_prompt");
        int (*unrelated)(void) = (int (*)(void))GetProcAddress(runtime, "fixture_other");
        void (*result)(void) = (void (*)(void))GetProcAddress(runtime, "fixture_result");
        if (show && unrelated && result) { prompt = show(); other = unrelated(); result(); }
    }
    char path[MAX_PATH], body[80];
    GetModuleFileNameA(NULL, path, MAX_PATH);
    char *name = strrchr(path, '\\');
    if (!name) return 1;
    strcpy(name + 1, "result.txt");
    int length = snprintf(body, sizeof body, "prompt=%d other=%d\n", prompt, other);
    HANDLE output = CreateFileA(path, GENERIC_WRITE, 0, NULL, CREATE_ALWAYS, FILE_ATTRIBUTE_NORMAL, NULL);
    if (output == INVALID_HANDLE_VALUE) return 2;
    DWORD written;
    WriteFile(output, body, (DWORD)length, &written, NULL);
    CloseHandle(output);
    return 0;
}
