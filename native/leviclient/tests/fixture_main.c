#include <windows.h>
#include <stdio.h>
static HANDLE heap_locked;
static LONG volatile heap_released;
static DWORD WINAPI hold_heap(void *unused) {
    (void)unused;
    HeapLock(GetProcessHeap());
    SetEvent(heap_locked);
    Sleep(500);
    InterlockedExchange(&heap_released, 1);
    HeapUnlock(GetProcessHeap());
    Sleep(1000);
    return 0;
}
int main(void) {
    HMODULE preloader = LoadLibraryW(L"PreLoader.dll");
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
    int guard = -1;
    char environment[8];
    if (preloader && GetEnvironmentVariableA("LEVI_GAME_AUTH_TOKEN", environment, sizeof environment)) {
        DWORD (*suspend)(HANDLE) = (DWORD (*)(HANDLE))GetProcAddress(preloader, "fixture_suspend");
        heap_locked = CreateEventW(NULL, TRUE, FALSE, NULL);
        HANDLE worker = CreateThread(NULL, 0, hold_heap, NULL, 0, NULL);
        WaitForSingleObject(heap_locked, INFINITE);
        DWORD count = suspend(worker);
        guard = count == 0 && InterlockedCompareExchange(&heap_released, 0, 0) == 1;
        if (guard) {
            void *allocation = HeapAlloc(GetProcessHeap(), 0, 1048576);
            guard = allocation != NULL;
            HeapFree(GetProcessHeap(), 0, allocation);
        }
        ResumeThread(worker);
        WaitForSingleObject(worker, INFINITE);
        CloseHandle(worker); CloseHandle(heap_locked);
    }
    char path[MAX_PATH], body[80];
    GetModuleFileNameA(NULL, path, MAX_PATH);
    char *name = strrchr(path, '\\');
    if (!name) return 1;
    strcpy(name + 1, "result.txt");
    int length = snprintf(body, sizeof body, "prompt=%d other=%d guard=%d\n", prompt, other, guard);
    HANDLE output = CreateFileA(path, GENERIC_WRITE, 0, NULL, CREATE_ALWAYS, FILE_ATTRIBUTE_NORMAL, NULL);
    if (output == INVALID_HANDLE_VALUE) return 2;
    DWORD written;
    WriteFile(output, body, (DWORD)length, &written, NULL);
    CloseHandle(output);
    return 0;
}
