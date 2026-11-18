#include <windows.h>
__declspec(dllimport) void *fopen(const char *, const char *);
__declspec(dllimport) int fputs(const char *, void *);
__declspec(dllimport) int fclose(void *);
__declspec(dllexport) int fixture_prompt(void) {
    return MessageBoxW(NULL, L"To sign in, enter the code\n\nAB12CD34\n\nContinue", L"Minecraft - Microsoft account sign-in", MB_OKCANCEL);
}
__declspec(dllexport) int fixture_other(void) {
    return MessageBoxW(NULL, L"Unrelated warning", L"Another dialog", MB_OKCANCEL);
}
__declspec(dllexport) void fixture_result(void) {
    void *file = fopen("C:\\xgr_signin.txt", "wb");
    fputs("Sign-in did not complete (hr=80004005)\r\n", file);
    fclose(file);
}
