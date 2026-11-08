// SPDX-License-Identifier: GPL-3.0-only
#include "winrt_activation.h"
#include "../core/logger.h"
#include <activation.h>
#include <appmodel.h>
#include <array>
#include <atomic>
#include <cwchar>
#include <string>
#include <winstring.h>

namespace levi {
namespace {
using GetFactoryFn = HRESULT(WINAPI *)(HSTRING, IActivationFactory **);

// Written once before the import slot points here, then only read.
std::wstring runtime_path;
std::atomic<GetFactoryFn> runtime_get_factory{nullptr};
std::atomic<bool> runtime_unavailable{false};

// Only Windows App SDK namespaces; Windows.* classes are never redirected.
bool is_app_sdk_class(HSTRING class_id) noexcept {
  constexpr std::wstring_view prefix = L"Microsoft.";
  UINT32 length = 0;
  const wchar_t *name = WindowsGetStringRawBuffer(class_id, &length);
  return name && length > prefix.size() && std::wmemcmp(name, prefix.data(), prefix.size()) == 0;
}

GetFactoryFn load_runtime() noexcept {
  if (const auto loaded = runtime_get_factory.load(std::memory_order_acquire))
    return loaded;
  if (runtime_unavailable.load(std::memory_order_acquire) || runtime_path.empty())
    return nullptr;
  // Never called under the loader lock: the game activates classes from its
  // own threads. A racing second load only takes another module reference.
  HMODULE module = LoadLibraryExW(runtime_path.c_str(), nullptr, LOAD_WITH_ALTERED_SEARCH_PATH);
  if (!module) {
    log(LogLevel::warning, "Cannot load the Windows App SDK runtime (Windows error {})",
        GetLastError());
    runtime_unavailable.store(true, std::memory_order_release);
    return nullptr;
  }
  const auto get_factory =
      reinterpret_cast<GetFactoryFn>(GetProcAddress(module, "DllGetActivationFactory"));
  if (!get_factory) {
    FreeLibrary(module);
    runtime_unavailable.store(true, std::memory_order_release);
    return nullptr;
  }
  // Keep the module loaded: factories handed to the game point into it.
  runtime_get_factory.store(get_factory, std::memory_order_release);
  return get_factory;
}

bool has_package_identity() noexcept {
  UINT32 length = 0;
  return GetCurrentPackageFullName(&length, nullptr) != APPMODEL_ERROR_NO_PACKAGE;
}
} // namespace

void set_app_runtime_path(const std::filesystem::path &path) {
  runtime_path = path.wstring();
  runtime_get_factory.store(nullptr, std::memory_order_release);
  runtime_unavailable.store(false, std::memory_order_release);
}

HRESULT WINAPI fallback_activation_factory(HSTRING class_id, REFIID iid, void **factory) noexcept {
  const HRESULT result = RoGetActivationFactory(class_id, iid, factory);
  if (result != REGDB_E_CLASSNOTREG || !factory || !is_app_sdk_class(class_id))
    return result;
  const auto get_factory = load_runtime();
  if (!get_factory)
    return result;
  IActivationFactory *activation = nullptr;
  if (FAILED(get_factory(class_id, &activation)) || !activation)
    return result;
  const HRESULT cast = activation->QueryInterface(iid, factory);
  activation->Release();
  return SUCCEEDED(cast) ? cast : result;
}

PatchResult install_winrt_activation_fallback(const std::filesystem::path &game_directory) noexcept {
  try {
    // A packaged launch resolves these classes through its framework package.
    if (has_package_identity())
      return {};
    const auto runtime = game_directory / app_runtime_dll_name;
    if (!std::filesystem::is_regular_file(runtime))
      return {};
    set_app_runtime_path(runtime);
  } catch (...) {
    return {0, true, ERROR_NOT_ENOUGH_MEMORY};
  }
  const std::array entries{ImportReplacement{
      "RoGetActivationFactory", reinterpret_cast<void *>(&fallback_activation_factory),
      reinterpret_cast<void *>(&RoGetActivationFactory)}};
  return patch_main_imports(entries);
}
} // namespace levi
