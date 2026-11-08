// SPDX-License-Identifier: GPL-3.0-only
#pragma once
#include "iat_hook.h"
#include <Windows.h>
#include <filesystem>
#include <roapi.h>

namespace levi {
// File name of the Windows App SDK runtime that GDK games ship next to the
// executable.
inline constexpr wchar_t app_runtime_dll_name[] = L"Microsoft.WindowsAppRuntime.dll";

// A packaged GDK game gets the Windows App SDK classes (for example
// Microsoft.Windows.Storage.Pickers.FileOpenPicker) from its framework package
// dependency. Started directly, the process has no package graph, so Windows
// cannot activate them. This routes the game's RoGetActivationFactory to the
// runtime DLL in the game directory when Windows reports such a class as not
// registered. Call from DllMain: it only patches the import table.
PatchResult install_winrt_activation_fallback(const std::filesystem::path &game_directory) noexcept;
// The replacement installed for RoGetActivationFactory. Exposed for tests.
HRESULT WINAPI fallback_activation_factory(HSTRING class_id, REFIID iid, void **factory) noexcept;
// Sets the runtime DLL fallback_activation_factory loads. Exposed for tests.
void set_app_runtime_path(const std::filesystem::path &path);
} // namespace levi
