// SPDX-License-Identifier: GPL-3.0-only
#pragma once
#include <filesystem>
#include <istream>
#include <string>

namespace levi {
struct VersionConfig {
  bool isolation = true;
  bool has_metadata = false;
  std::string game_version;
  std::string channel;
};
VersionConfig parse_version_config(std::istream &input);
VersionConfig read_version_config(const std::filesystem::path &directory);
std::filesystem::path legacy_data_directory(const std::filesystem::path &game,
                                            const VersionConfig &config);
} // namespace levi
