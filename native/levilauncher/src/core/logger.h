// SPDX-License-Identifier: GPL-3.0-only
#pragma once
#include <format>
#include <string_view>
#include <utility>

namespace levi {
enum class LogLevel { info, warning, error };
void write_log(LogLevel level, std::string_view message) noexcept;
template <typename... Args>
void log(LogLevel level, std::format_string<Args...> format, Args &&...args) noexcept {
  try {
    write_log(level, std::format(format, std::forward<Args>(args)...));
  } catch (...) {
    write_log(LogLevel::error, "Failed to format log message");
  }
}
} // namespace levi
