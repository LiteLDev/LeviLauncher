// SPDX-License-Identifier: GPL-3.0-only
#include <ShlObj.h>
#include <Windows.h>
#include <filesystem>
#include <string>
#include <vector>

extern "C" __declspec(dllimport) void LeviLauncherEntry() noexcept;
int wmain(int argc, wchar_t **argv) {
  LeviLauncherEntry();
  if (argc != 3)
    return 1;
  const bool isolation = std::wstring(argv[1]) == L"on";
  std::vector<wchar_t> path(32768);
  const auto length = GetModuleFileNameW(nullptr, path.data(), static_cast<DWORD>(path.size()));
  const auto directory = std::filesystem::path(std::wstring(path.data(), length)).parent_path();
  PWSTR local = nullptr;
  if (FAILED(SHGetKnownFolderPath(FOLDERID_LocalAppData, 0, nullptr, &local)))
    return 2;
  const bool empty_local = local && *local == L'\0';
  CoTaskMemFree(local);
  if (!empty_local)
    return 3;
  PWSTR roaming = nullptr;
  if (FAILED(SHGetKnownFolderPath(FOLDERID_RoamingAppData, 0, nullptr, &roaming)))
    return 4;
  const bool redirected = std::filesystem::path(roaming) == directory;
  CoTaskMemFree(roaming);
  if (redirected != isolation)
    return 5;
  const DWORD required = GetTempPathW(0, nullptr);
  if (required == 0 || required > path.size())
    return 6;
  const DWORD copied = GetTempPathW(static_cast<DWORD>(path.size()), path.data());
  if (copied == 0 || copied >= path.size() || path[copied - 1] != L'\\')
    return 7;
  if (isolation) {
    const auto expected_directory = *argv[2] ? directory / argv[2] : directory;
    const auto expected = expected_directory.wstring() + L"\\";
    if (std::wstring(path.data()) != expected)
      return 8;
  } else {
    // Bypass the IAT so the expected path cannot follow an accidental hook.
    const auto system_temp = reinterpret_cast<decltype(&GetTempPathW)>(
        GetProcAddress(GetModuleHandleW(L"kernel32.dll"), "GetTempPathW"));
    if (!system_temp)
      return 8;
    std::vector<wchar_t> expected(path.size());
    const DWORD expected_size = system_temp(static_cast<DWORD>(expected.size()), expected.data());
    if (expected_size == 0 || expected_size >= expected.size() || copied != expected_size ||
        std::wstring(path.data()) != expected.data())
      return 8;
  }
  if (std::filesystem::exists(directory / L"TEMP"))
    return 8;
  for (int i = 0; i < 500; ++i) {
    if (GetModuleHandleW(L"native_fixture.dll"))
      return 0;
    Sleep(10);
  }
  return 9;
}
