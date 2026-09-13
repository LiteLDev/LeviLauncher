// SPDX-License-Identifier: GPL-3.0-only
#pragma once
#include <Windows.h>
#include <filesystem>
#include <string>
#include <string_view>

namespace levi {
std::filesystem::path module_path(HMODULE module);
std::wstring from_utf8(std::string_view value);
std::string to_utf8(std::wstring_view value);
std::string to_ansi(std::wstring_view value);
std::wstring with_separator(const std::filesystem::path &directory);

// Required capacity includes NUL; successful length excludes it. Never truncate.
template <typename Char>
DWORD copy_path(std::basic_string_view<Char> path, DWORD capacity, Char *output) noexcept {
  const auto required = static_cast<DWORD>(path.size() + 1);
  if (capacity < required)
    return required;
  if (!output) {
    SetLastError(ERROR_INVALID_PARAMETER);
    return 0;
  }
  std::char_traits<Char>::copy(output, path.data(), path.size());
  output[path.size()] = Char{};
  return required - 1;
}
} // namespace levi
