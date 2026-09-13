// SPDX-License-Identifier: GPL-3.0-only
#pragma once
#include <filesystem>
#include <istream>
#include <string>
#include <string_view>

namespace levi {
struct VersionConfig {
  bool isolation = true;
  bool has_metadata = false;
  bool uwp = false;
  bool console = false;
  std::string game_version;
  std::string channel;
};
VersionConfig parse_version_config(std::istream &input);
VersionConfig read_version_config(const std::filesystem::path &directory);
bool supports_uwp_isolation(std::string_view game_version);
std::filesystem::path legacy_data_directory(const std::filesystem::path &game,
                                            const VersionConfig &config);
std::wstring channel_directory_name(const std::string &channel);
} // namespace levi
