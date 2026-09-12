// SPDX-License-Identifier: GPL-3.0-only
#include "windows_paths.h"
#include <algorithm>
#include <limits>
#include <stdexcept>
#include <system_error>
#include <vector>

namespace levi {
namespace {
int checked_length(std::size_t size) {
  if (size > static_cast<std::size_t>(std::numeric_limits<int>::max())) {
    throw std::length_error("Windows string exceeds INT_MAX");
  }
  return static_cast<int>(size);
}
std::string encode(std::wstring_view value, UINT code_page, DWORD flags) {
  if (value.empty())
    return {};
  BOOL substituted = FALSE;
  const bool utf8 = code_page == CP_UTF8 || (code_page == CP_ACP && GetACP() == CP_UTF8);
  BOOL *used_default = utf8 ? nullptr : &substituted;
  if (utf8)
    flags = WC_ERR_INVALID_CHARS;
  const auto length = checked_length(value.size());
  const int size = WideCharToMultiByte(code_page, flags, value.data(), length, nullptr, 0, nullptr,
                                       used_default);
  if (!size || substituted) {
    throw std::system_error(ERROR_NO_UNICODE_TRANSLATION, std::system_category());
  }
  std::string result(size, '\0');
  if (!WideCharToMultiByte(code_page, flags, value.data(), length, result.data(), size, nullptr,
                           used_default) ||
      substituted) {
    throw std::system_error(ERROR_NO_UNICODE_TRANSLATION, std::system_category());
  }
  return result;
}
} // namespace
std::filesystem::path module_path(HMODULE module) {
  for (DWORD capacity = 260; capacity <= 32768; capacity = std::min(capacity * 2, 32768UL)) {
    std::vector<wchar_t> buffer(capacity);
    const DWORD size = GetModuleFileNameW(module, buffer.data(), capacity);
    if (!size) {
      throw std::system_error(GetLastError(), std::system_category(), "GetModuleFileNameW");
    }
    if (size < capacity)
      return std::wstring(buffer.data(), size);
    if (capacity == 32768)
      break;
  }
  throw std::length_error("Module path exceeds Windows path limit");
}
std::wstring from_utf8(std::string_view value) {
  if (value.empty())
    return {};
  const auto length = checked_length(value.size());
  const int size =
      MultiByteToWideChar(CP_UTF8, MB_ERR_INVALID_CHARS, value.data(), length, nullptr, 0);
  if (!size)
    throw std::system_error(GetLastError(), std::system_category(), "Invalid UTF-8");
  std::wstring result(size, L'\0');
  if (!MultiByteToWideChar(CP_UTF8, MB_ERR_INVALID_CHARS, value.data(), length, result.data(),
                           size)) {
    throw std::system_error(GetLastError(), std::system_category(), "MultiByteToWideChar");
  }
  return result;
}
std::string to_utf8(std::wstring_view value) {
  return encode(value, CP_UTF8, WC_ERR_INVALID_CHARS);
}
std::string to_ansi(std::wstring_view value) { return encode(value, CP_ACP, WC_NO_BEST_FIT_CHARS); }
std::wstring with_separator(const std::filesystem::path &directory) {
  auto value = directory.wstring();
  if (!value.empty() && value.back() != L'\\' && value.back() != L'/')
    value += L'\\';
  return value;
}
} // namespace levi
