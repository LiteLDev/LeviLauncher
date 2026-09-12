// SPDX-License-Identifier: GPL-3.0-only
#include "logger.h"
#include "../utils/windows_paths.h"
#include <Windows.h>

namespace levi {
void write_log(LogLevel level, std::string_view message) noexcept {
  try {
    SYSTEMTIME time{};
    GetLocalTime(&time);
    const auto label = level == LogLevel::error     ? "ERROR"
                       : level == LogLevel::warning ? "WARN"
                                                    : "INFO";
    const auto text = std::format("{:02}:{:02}:{:02}.{:03} [{}] [LeviLauncher] {}\r\n", time.wHour,
                                  time.wMinute, time.wSecond, time.wMilliseconds, label, message);
    const auto wide = from_utf8(text);
    OutputDebugStringW(wide.c_str());
    const HANDLE output = GetStdHandle(STD_OUTPUT_HANDLE);
    if (!output || output == INVALID_HANDLE_VALUE)
      return;
    DWORD mode = 0;
    DWORD written = 0;
    if (GetConsoleMode(output, &mode)) {
      WriteConsoleW(output, wide.data(), static_cast<DWORD>(wide.size()), &written, nullptr);
    } else {
      WriteFile(output, text.data(), static_cast<DWORD>(text.size()), &written, nullptr);
    }
  } catch (...) {
    OutputDebugStringW(L"[LeviLauncher] Logging failed\n");
  }
}
} // namespace levi
