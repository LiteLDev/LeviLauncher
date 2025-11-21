#include <windows.h>
#include <string>

int wmain(int argc, wchar_t* argv[]) {
    if (argc < 3) return 2;
    std::wstring msix = argv[1];
    std::wstring out = argv[2];

    wchar_t dllPath[MAX_PATH];
    std::wstring envdll;
    wchar_t* envp = _wgetenv(L"LAUNCHER_CORE_DLL");
    if (envp && *envp) envdll = envp;
    if (!envdll.empty()) {
        wcsncpy_s(dllPath, envdll.c_str(), _TRUNCATE);
    } else {
        wchar_t exePath[MAX_PATH];
        GetModuleFileNameW(NULL, exePath, MAX_PATH);
        wchar_t dir[MAX_PATH];
        wcsncpy_s(dir, exePath, _TRUNCATE);
        for (int i = (int)wcslen(dir)-1; i >= 0; --i) { if (dir[i] == L'\\' || dir[i] == L'/') { dir[i] = 0; break; } }
        std::wstring cand = std::wstring(dir) + L"\\launcher_core.dll";
        WIN32_FILE_ATTRIBUTE_DATA fad;
        if (GetFileAttributesExW(cand.c_str(), GetFileExInfoStandard, &fad)) {
            wcsncpy_s(dllPath, cand.c_str(), _TRUNCATE);
        } else {
            wcsncpy_s(dllPath, L"launcher_core.dll", _TRUNCATE);
        }
    }

    HMODULE h = LoadLibraryW(dllPath);
    if (!h) return 3;
    typedef int (__stdcall *MIW)(const wchar_t*, const wchar_t*);
    typedef int (__stdcall *MIA)(const char*, const char*);
    MIW miw = (MIW)GetProcAddress(h, "miHoYoW");
    if (miw) {
        int rc = miw(msix.c_str(), out.c_str());
        FreeLibrary(h);
        return rc;
    }
    MIA mia = (MIA)GetProcAddress(h, "miHoYo");
    if (mia) {
        int rc = mia(std::string(msix.begin(), msix.end()).c_str(), std::string(out.begin(), out.end()).c_str());
        FreeLibrary(h);
        return rc;
    }
    FreeLibrary(h);
    return 3;
}