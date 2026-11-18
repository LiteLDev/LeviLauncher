/* Test imports only: no windows are displayed and no status files are written. */
#include <windows.h>
int WINAPI fixture_message_box(HWND owner, LPCWSTR text, LPCWSTR title, UINT flags) {
    (void)owner; (void)text; (void)title; (void)flags;
    return IDCANCEL;
}
void *fixture_open(const char *path, const char *mode) { (void)path; (void)mode; return (void *)0x1234; }
int fixture_put(const char *text, void *file) { (void)text; (void)file; return 1; }
int fixture_close(void *file) { (void)file; return 0; }
