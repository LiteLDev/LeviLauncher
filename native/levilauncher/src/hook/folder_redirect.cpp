// SPDX-License-Identifier: GPL-3.0-only
#include "folder_redirect.h"
#include "../core/logger.h"
#include "../utils/windows_paths.h"
#include <array>
#include <cstring>
#include <system_error>

namespace levi {
namespace {
const RedirectPaths *active_paths = nullptr;
HRESULT allocate_path(std::wstring_view value, PWSTR *output) noexcept {
  if (!output)
    return E_INVALIDARG;
  *output = nullptr;
  const auto bytes = (value.size() + 1) * sizeof(wchar_t);
  auto *copy = static_cast<PWSTR>(CoTaskMemAlloc(bytes));
  if (!copy)
    return E_OUTOFMEMORY;
  std::memcpy(copy, value.data(), value.size() * sizeof(wchar_t));
  copy[value.size()] = L'\0';
  *output = copy;
  return S_OK;
}
} // namespace

RedirectPaths prepare_redirect_paths(const std::filesystem::path &game,
                                     const VersionConfig &config) {
  RedirectPaths paths;
  paths.isolation = config.isolation;
  paths.roaming = game.wstring();
  if (!config.isolation)
    return paths;
  const auto data_directory = legacy_data_directory(game, config);
  std::filesystem::create_directories(data_directory);
  paths.temp_wide = with_separator(data_directory);
  try {
    paths.temp_ansi = to_ansi(paths.temp_wide);
  } catch (const std::system_error &) {
    // Never substitute '?' and redirect the game to a different directory.
    log(LogLevel::warning, "Isolated ANSI path is not representable in the system code page");
  }
  return paths;
}

HRESULT WINAPI redirected_known_folder(REFKNOWNFOLDERID id, DWORD flags, HANDLE token,
                                       PWSTR *output) noexcept {
  if (!output)
    return E_INVALIDARG;
  if (active_paths && IsEqualGUID(id, FOLDERID_LocalAppData))
    return allocate_path(L"", output);
  if (active_paths && active_paths->isolation && IsEqualGUID(id, FOLDERID_RoamingAppData)) {
    return allocate_path(active_paths->roaming, output);
  }
  return SHGetKnownFolderPath(id, flags, token, output);
}
DWORD WINAPI redirected_temp_a(DWORD capacity, LPSTR output) noexcept {
  if (!active_paths || !active_paths->isolation)
    return GetTempPathA(capacity, output);
  if (active_paths->temp_ansi.empty()) {
    SetLastError(ERROR_NO_UNICODE_TRANSLATION);
    return 0;
  }
  return copy_path<char>(active_paths->temp_ansi, capacity, output);
}
DWORD WINAPI redirected_temp_w(DWORD capacity, LPWSTR output) noexcept {
  if (!active_paths || !active_paths->isolation)
    return GetTempPathW(capacity, output);
  return copy_path<wchar_t>(active_paths->temp_wide, capacity, output);
}
PatchResult install_folder_redirects(const RedirectPaths &paths) noexcept {
  active_paths = &paths;
  // Both temp-path APIs use the same version-specific data directory.
  const std::array entries{
      ImportReplacement{"SHGetKnownFolderPath", reinterpret_cast<void *>(&redirected_known_folder),
                        reinterpret_cast<void *>(&SHGetKnownFolderPath)},
      ImportReplacement{"GetTempPathA", reinterpret_cast<void *>(&redirected_temp_a),
                        reinterpret_cast<void *>(&GetTempPathA)},
      ImportReplacement{"GetTempPathW", reinterpret_cast<void *>(&redirected_temp_w),
                        reinterpret_cast<void *>(&GetTempPathW)}};
  return patch_main_imports(entries);
}
} // namespace levi
