// SPDX-License-Identifier: GPL-3.0-only
#pragma once
#include <filesystem>
#include <vector>

namespace levi {
struct ModLoadResult {
  unsigned loaded = 0;
  unsigned failed = 0;
  bool external_preloader = false;
};
std::filesystem::path resolve_mod_entry(const std::filesystem::path &directory,
                                        const std::string &entry);
ModLoadResult load_native_mods(const std::filesystem::path &loader_directory) noexcept;
} // namespace levi
