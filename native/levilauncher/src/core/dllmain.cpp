// SPDX-License-Identifier: GPL-3.0-only
#include "../config/mod_loader.h"
#include "../config/version_config.h"
#include "../hook/folder_redirect.h"
#include "../utils/windows_paths.h"
#include "logger.h"
#include <Windows.h>
#include <cstdio>
#include <memory>

namespace {
struct Runtime {
  std::filesystem::path loader_directory;
  levi::RedirectPaths redirects;
  bool console = false;
};

DWORD WINAPI load_mods(void *context) noexcept {
  const auto &runtime = *static_cast<const Runtime *>(context);
  // UWP activation does not create a console from the PE subsystem. Allocate
  // it after leaving DllMain, before any Mod writes to its standard streams.
  if (runtime.console) {
    if (AllocConsole()) {
      FILE *stream = nullptr;
      (void)freopen_s(&stream, "CONIN$", "r", stdin);
      (void)freopen_s(&stream, "CONOUT$", "w", stdout);
      (void)freopen_s(&stream, "CONOUT$", "w", stderr);
      SetConsoleCP(CP_UTF8);
      SetConsoleOutputCP(CP_UTF8);
    } else if (GetLastError() != ERROR_ACCESS_DENIED) {
      levi::log(levi::LogLevel::error, "Cannot allocate console (Windows error {})", GetLastError());
    }
  }
  levi::load_native_mods(runtime.loader_directory);
  return 0;
}

bool attach(HMODULE module) noexcept {
  try {
    const auto executable = levi::module_path(nullptr);
    if (_wcsicmp(executable.filename().c_str(), L"Minecraft.Windows.exe") != 0 &&
        _wcsicmp(executable.filename().c_str(), L"Minecraft.Win10.DX11.exe") != 0)
      return true;

    auto runtime = std::make_unique<Runtime>();
    runtime->loader_directory = levi::module_path(module).parent_path();
    // Isolation must be ready before the game's first folder query. Only local
    // file I/O and Kernel32 APIs are used here; Mod loading runs after DllMain.
    auto config = levi::read_version_config(runtime->loader_directory);
    // Imported historical instances can lack metadata. GDK can also have an
    // AppxManifest, but always has MicrosoftGame.config.
    const bool uwp = config.uwp ||
        (std::filesystem::is_regular_file(runtime->loader_directory / L"AppxManifest.xml") &&
         !std::filesystem::is_regular_file(runtime->loader_directory / L"MicrosoftGame.config"));
    runtime->console = uwp && config.console;
    if (!uwp)
      runtime->redirects = levi::prepare_redirect_paths(executable.parent_path(), config);

    HMODULE pinned = nullptr;
    if (!GetModuleHandleExW(GET_MODULE_HANDLE_EX_FLAG_FROM_ADDRESS | GET_MODULE_HANDLE_EX_FLAG_PIN,
                            reinterpret_cast<LPCWSTR>(module), &pinned))
      return false;

    // Once an IAT slot can point here, both the DLL and its immutable state must
    // live until process exit. Do not free them during DLL_PROCESS_DETACH.
    const auto *state = runtime.release();
    // GDK redirects include an unconditional LocalAppData override. UWP must
    // keep every Windows package data path, even with stale isolation metadata.
    if (!uwp) {
      const auto patched = levi::install_folder_redirects(state->redirects);
      if (!patched.valid_image || patched.error != ERROR_SUCCESS) {
        levi::log(levi::LogLevel::error, "Cannot install directory imports (Windows error {})",
                  patched.error);
        return false;
      }
    }
    HANDLE thread = CreateThread(nullptr, 0, load_mods, const_cast<Runtime *>(state), 0, nullptr);
    if (!thread) {
      levi::log(levi::LogLevel::error, "Cannot start Mod loader (Windows error {})",
                GetLastError());
      return false;
    }
    CloseHandle(thread); // Never wait for a worker while holding the loader lock.
    return true;
  } catch (const std::exception &e) {
    levi::log(levi::LogLevel::error, "Initialization failed: {}", e.what());
    return false;
  } catch (...) {
    levi::write_log(levi::LogLevel::error, "Unexpected initialization failure");
    return false;
  }
}
} // namespace

BOOL APIENTRY DllMain(HMODULE module, DWORD reason, LPVOID) {
  if (reason != DLL_PROCESS_ATTACH)
    return TRUE;
  DisableThreadLibraryCalls(module);
  return attach(module) ? TRUE : FALSE;
}
