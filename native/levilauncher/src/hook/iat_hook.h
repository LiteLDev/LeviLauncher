// SPDX-License-Identifier: GPL-3.0-only
#pragma once
#include <Windows.h>
#include <cstddef>
#include <span>
#include <string_view>

namespace levi {
struct ImportReplacement {
  std::string_view name;
  void *replacement;
  void *original; // Also matches images whose OriginalFirstThunk is absent.
};
struct PatchResult {
  std::size_t patched = 0;
  bool valid_image = true;
  DWORD error = ERROR_SUCCESS;
};
PatchResult patch_imports(std::span<std::byte> image,
                          std::span<const ImportReplacement> replacements) noexcept;
PatchResult patch_main_imports(std::span<const ImportReplacement> replacements) noexcept;
} // namespace levi
