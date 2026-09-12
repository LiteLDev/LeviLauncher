// SPDX-License-Identifier: GPL-3.0-only
#include "mod_loader.h"
#include "../core/logger.h"
#include "../utils/windows_paths.h"
#include <Windows.h>
#include <algorithm>
#include <fstream>
#include <nlohmann/json.hpp>
#include <set>
#include <stdexcept>

namespace levi {
std::filesystem::path resolve_mod_entry(const std::filesystem::path &directory,
                                        const std::string &entry) {
  if (entry.empty() || entry.find('\0') != std::string::npos ||
      entry.find(':') != std::string::npos) {
    throw std::invalid_argument("Mod entry must be a relative DLL path");
  }
  const std::filesystem::path relative(from_utf8(entry));
  if (relative.is_absolute() || relative.has_root_name() || relative.has_root_directory()) {
    throw std::invalid_argument("Mod entry must be relative");
  }
  const auto base = std::filesystem::canonical(directory);
  const auto resolved = std::filesystem::canonical(base / relative);
  auto candidate = resolved.begin();
  for (const auto &part : base) {
    if (candidate == resolved.end() || _wcsicmp(part.c_str(), candidate->c_str()) != 0) {
      throw std::invalid_argument("Mod entry escapes its directory");
    }
    ++candidate;
  }
  if (candidate == resolved.end() || _wcsicmp(resolved.extension().c_str(), L".dll") != 0 ||
      !std::filesystem::is_regular_file(resolved)) {
    throw std::invalid_argument("Mod entry must be an existing DLL");
  }
  return resolved;
}

ModLoadResult load_native_mods(const std::filesystem::path &loader_directory) noexcept {
  ModLoadResult result;
  try {
    // The external preloader owns all Mod loading when present.
    if (std::filesystem::is_regular_file(loader_directory / L"preloader.dll")) {
      result.external_preloader = true;
      log(LogLevel::info, "External preloader detected; skipping native Mods");
      return result;
    }
    const auto mods = loader_directory / L"mods";
    std::filesystem::create_directories(mods);
    std::vector<std::filesystem::path> directories;
    for (const auto &entry : std::filesystem::directory_iterator(mods)) {
      if (entry.is_directory())
        directories.push_back(entry.path());
    }
    std::sort(directories.begin(), directories.end());
    std::set<std::filesystem::path> loaded;
    for (const auto &directory : directories) {
      try {
        std::ifstream input(directory / L"manifest.json");
        if (!input)
          continue;
        const auto manifest = nlohmann::json::parse(input);
        if (!manifest.is_object())
          throw std::invalid_argument("Mod manifest must contain an object");
        if (manifest.value("type", std::string{}) != "preload-native")
          continue;
        const auto entry = resolve_mod_entry(directory, manifest.at("entry").get<std::string>());
        if (loaded.contains(entry))
          continue;
        if (!LoadLibraryExW(entry.c_str(), nullptr, LOAD_WITH_ALTERED_SEARCH_PATH)) {
          const DWORD code = GetLastError();
          ++result.failed;
          log(LogLevel::error, "Cannot load {} (Windows error {})", to_utf8(entry.wstring()), code);
          continue;
        }
        loaded.insert(entry); // Loaded modules intentionally live until process exit.
        ++result.loaded;
        log(LogLevel::info, "Loaded {}", to_utf8(entry.wstring()));
      } catch (const std::exception &e) {
        ++result.failed;
        log(LogLevel::error, "Invalid Mod at {}: {}", to_utf8(directory.wstring()), e.what());
      }
    }
  } catch (const std::exception &e) {
    ++result.failed;
    log(LogLevel::error, "Cannot enumerate native Mods: {}", e.what());
  } catch (...) {
    ++result.failed;
    write_log(LogLevel::error, "Unexpected native Mod loader failure");
  }
  return result;
}
} // namespace levi
