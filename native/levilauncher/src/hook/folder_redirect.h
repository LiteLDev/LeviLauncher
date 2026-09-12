// SPDX-License-Identifier: GPL-3.0-only
#pragma once
#include "../config/version_config.h"
#include "iat_hook.h"
#include <ShlObj.h>
#include <Windows.h>
#include <filesystem>
#include <string>

namespace levi {
struct RedirectPaths {
  bool isolation = true;
  std::wstring roaming;
  std::wstring temp_wide;
  std::string temp_ansi;
};
RedirectPaths prepare_redirect_paths(const std::filesystem::path &game,
                                     const VersionConfig &config);
// paths must outlive the process's installed import slots.
PatchResult install_folder_redirects(const RedirectPaths &paths) noexcept;
HRESULT WINAPI redirected_known_folder(REFKNOWNFOLDERID id, DWORD flags, HANDLE token,
                                       PWSTR *output) noexcept;
DWORD WINAPI redirected_temp_a(DWORD capacity, LPSTR output) noexcept;
DWORD WINAPI redirected_temp_w(DWORD capacity, LPWSTR output) noexcept;
} // namespace levi
