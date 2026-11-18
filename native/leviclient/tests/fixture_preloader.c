#include <windows.h>
__declspec(dllexport) DWORD fixture_suspend(HANDLE thread) { return SuspendThread(thread); }
