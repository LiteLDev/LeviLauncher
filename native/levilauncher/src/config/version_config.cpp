// SPDX-License-Identifier: GPL-3.0-only
#include "version_config.h"
#include "../core/logger.h"
#include <array>
#include <charconv>
#include <fstream>
#include <nlohmann/json.hpp>
#include <optional>
#include <string_view>

namespace levi {
namespace {
std::optional<std::array<unsigned, 4>> parse_version(std::string_view text) {
  std::array<unsigned, 4> parts{};
  for (std::size_t i = 0; i < parts.size(); ++i) {
    const auto end = text.find('.');
    const auto part = text.substr(0, end);
    const auto result = std::from_chars(part.data(), part.data() + part.size(), parts[i]);
    if (part.empty() || result.ec != std::errc{} || result.ptr != part.data() + part.size()) {
      return std::nullopt;
    }
    if (end == std::string_view::npos)
      return parts;
    text.remove_prefix(end + 1);
  }
  return std::nullopt;
}
bool read_isolation(const nlohmann::json &value) {
  if (value.is_boolean())
    return value.get<bool>();
  if (value.is_number_integer())
    return value != 0;
  if (value.is_string()) {
    auto text = value.get<std::string>();
    for (char &ch : text) {
      if (ch >= 'A' && ch <= 'Z')
        ch = static_cast<char>(ch + ('a' - 'A'));
    }
    return text != "false";
  }
  return true;
}
} // namespace

VersionConfig parse_version_config(std::istream &input) {
  const auto json = nlohmann::json::parse(input);
  if (!json.is_object())
    throw std::invalid_argument("version.json must contain an object");
  VersionConfig config;
  config.has_metadata = true;
  if (const auto it = json.find("packageType"); it != json.end() && it->is_string()) {
    auto type = it->get<std::string>();
    for (char &ch : type) {
      if (ch >= 'A' && ch <= 'Z')
        ch = static_cast<char>(ch + ('a' - 'A'));
    }
    config.uwp = type == "uwp";
  }
  if (const auto it = json.find("enableConsole"); it != json.end() && it->is_boolean()) {
    config.console = it->get<bool>();
  }
  const bool has_isolation = json.find("enableIsolation") != json.end();
  if (const auto it = json.find("enableIsolation"); it != json.end()) {
    config.isolation = read_isolation(*it);
  }
  if (const auto it = json.find("gameVersion"); it != json.end() && it->is_string()) {
    config.game_version = it->get<std::string>();
  }
  if (const auto it = json.find("type"); it != json.end() && it->is_string()) {
    config.channel = it->get<std::string>();
  }
  // UWP isolation redirects LocalFolder only and is opt-in; missing metadata keeps
  // the shared package data path.
  if (config.uwp && (!has_isolation || !supports_uwp_isolation(config.game_version)))
    config.isolation = false;
  return config;
}
bool supports_uwp_isolation(std::string_view game_version) {
  const auto first = game_version.find_first_not_of(" \t\r\n");
  if (first == std::string_view::npos)
    return false;
  const auto last = game_version.find_last_not_of(" \t\r\n");
  const auto version = parse_version(game_version.substr(first, last - first + 1));
  return version && *version >= std::array<unsigned, 4>{1, 19, 70, 2};
}
VersionConfig read_version_config(const std::filesystem::path &directory) {
  std::ifstream input(directory / L"version.json");
  if (!input)
    return {};
  try {
    return parse_version_config(input);
  } catch (const std::exception &e) {
    log(LogLevel::warning, "Cannot read version.json; using defaults: {}", e.what());
    return {};
  }
}
std::filesystem::path legacy_data_directory(const std::filesystem::path &game,
                                            const VersionConfig &config) {
  if (!config.has_metadata)
    return game;
  if (config.game_version == "1.26.0.24")
    return game / L"Minecraft Bedrock Preview";
  const auto version = parse_version(config.game_version);
  if (version && *version >= std::array<unsigned, 4>{1, 26, 0, 0})
    return game;
  return game / (config.channel == "preview" ? L"Minecraft Bedrock Preview" : L"Minecraft Bedrock");
}
std::wstring channel_directory_name(const std::string &channel) {
  std::string lower = channel;
  for (char &ch : lower) {
    if (ch >= 'A' && ch <= 'Z')
      ch = static_cast<char>(ch + ('a' - 'A'));
  }
  if (lower == "preview")
    return L"Minecraft Bedrock Preview";
  if (lower == "beta")
    return L"Minecraft Bedrock Beta";
  return L"Minecraft Bedrock";
}
} // namespace levi
