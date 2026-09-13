// SPDX-License-Identifier: GPL-3.0-only
#include "folder_redirect.h"
#include "../core/logger.h"
#include "../utils/windows_paths.h"
#include <array>
#include <cstring>
#include <roapi.h>
#include <system_error>
#include <windows.storage.h>
#include <wrl/client.h>
#include <wrl/wrappers/corewrappers.h>

namespace levi {
namespace {
const RedirectPaths *active_paths = nullptr;
using GetLocalFolderFn = HRESULT(STDMETHODCALLTYPE *)(
    ABI::Windows::Storage::IApplicationData *This, ABI::Windows::Storage::IStorageFolder **value);
GetLocalFolderFn original_get_local_folder = nullptr;

// COM vtable slots for the ApplicationData ABI interfaces. A published WinRT
// interface is immutable, so these indices are stable across SDK versions.
// IInspectable occupies slots 0-5; interface methods follow in IDL order (see
// windows.storage.h). IApplicationData: get_LocalFolder is slot 12,
// get_TemporaryFolder slot 14. IApplicationData2: get_LocalCacheFolder is slot 6.
constexpr std::size_t get_local_folder_index = 12;
constexpr std::size_t get_temporary_folder_index = 14;
constexpr std::size_t get_local_cache_folder_index = 6;

HRESULT allocate_path(std::wstring_view value, PWSTR *output) noexcept {
  if (!output)
    return E_INVALIDARG;
  *output = nullptr;
  const auto bytes = (value.size() + 1) * sizeof(wchar_t);
  auto *copy = static_cast<PWSTR>(CoTaskMemAlloc(bytes));
  if (!copy)
    return E_OUTOFMEMORY;
  std::memcpy(copy, value.data(), value.size() * sizeof(wchar_t));
  copy[value.size()] = L'\0';
  *output = copy;
  return S_OK;
}

void replace_vtable_slot(void *instance, std::size_t index, void *replacement,
                         void **original) noexcept {
  auto **vtable = *static_cast<void ***>(instance);
  void *current = vtable[index];
  if (original)
    *original = current;
  if (current == replacement)
    return;
  DWORD previous = 0;
  if (!VirtualProtect(vtable + index, sizeof(void *), PAGE_READWRITE, &previous))
    return;
  InterlockedExchangePointer(reinterpret_cast<void *volatile *>(vtable + index), replacement);
  DWORD ignored = 0;
  VirtualProtect(vtable + index, sizeof(void *), previous, &ignored);
}

HRESULT blocking_folder_from_path(std::wstring_view path,
                                  ABI::Windows::Storage::IStorageFolder **folder) noexcept {
  using Microsoft::WRL::ComPtr;
  using Microsoft::WRL::Wrappers::HString;
  using Microsoft::WRL::Wrappers::HStringReference;

  *folder = nullptr;
  ComPtr<ABI::Windows::Storage::IStorageFolderStatics> statics;
  HRESULT hr = Windows::Foundation::GetActivationFactory(
      HStringReference(RuntimeClass_Windows_Storage_StorageFolder).Get(), &statics);
  if (FAILED(hr))
    return hr;

  HString value;
  hr = value.Set(path.data(), static_cast<unsigned int>(path.size()));
  if (FAILED(hr))
    return hr;

  ComPtr<__FIAsyncOperation_1_Windows__CStorage__CStorageFolder> operation;
  hr = statics->GetFolderFromPathAsync(value.Get(), &operation);
  if (FAILED(hr))
    return hr;

  ComPtr<ABI::Windows::Foundation::IAsyncInfo> info;
  hr = operation.As(&info);
  if (FAILED(hr))
    return hr;

  for (;;) {
    ABI::Windows::Foundation::AsyncStatus status{};
    hr = info->get_Status(&status);
    if (FAILED(hr))
      return hr;
    if (status == ABI::Windows::Foundation::AsyncStatus::Completed)
      break;
    if (status != ABI::Windows::Foundation::AsyncStatus::Started)
      return E_FAIL;
    Sleep(1);
  }

  ComPtr<ABI::Windows::Storage::IStorageFolder> result;
  hr = operation->GetResults(&result);
  if (FAILED(hr))
    return hr;
  *folder = result.Detach();
  return S_OK;
}

// Backs LocalFolder, TemporaryFolder and LocalCacheFolder. All three resolve to
// the instance's data directory so an isolated UWP instance keeps its worlds,
// settings, logs and caches together, matching GDK isolation.
HRESULT STDMETHODCALLTYPE
redirected_folder(ABI::Windows::Storage::IApplicationData *This,
                  ABI::Windows::Storage::IStorageFolder **value) noexcept {
  if (!value)
    return E_INVALIDARG;
  *value = nullptr;
  if (!active_paths || !active_paths->isolation || active_paths->local_folder.empty()) {
    if (!original_get_local_folder)
      return E_UNEXPECTED;
    return original_get_local_folder(This, value);
  }
  return blocking_folder_from_path(active_paths->local_folder, value);
}
} // namespace

RedirectPaths prepare_redirect_paths(const std::filesystem::path &game,
                                     const VersionConfig &config) {
  RedirectPaths paths;
  paths.isolation = config.isolation;
  paths.roaming = game.wstring();
  paths.local_folder = game.wstring();
  if (!config.isolation)
    return paths;
  const auto data_directory = legacy_data_directory(game, config);
  std::filesystem::create_directories(data_directory);
  paths.temp_wide = with_separator(data_directory);
  try {
    paths.temp_ansi = to_ansi(paths.temp_wide);
  } catch (const std::system_error &) {
    // Never substitute '?' and redirect the game to a different directory.
    log(LogLevel::warning, "Isolated ANSI path is not representable in the system code page");
  }
  return paths;
}

HRESULT WINAPI redirected_known_folder(REFKNOWNFOLDERID id, DWORD flags, HANDLE token,
                                       PWSTR *output) noexcept {
  if (!output)
    return E_INVALIDARG;
  if (active_paths && IsEqualGUID(id, FOLDERID_LocalAppData))
    return allocate_path(L"", output);
  if (active_paths && active_paths->isolation && IsEqualGUID(id, FOLDERID_RoamingAppData)) {
    return allocate_path(active_paths->roaming, output);
  }
  return SHGetKnownFolderPath(id, flags, token, output);
}
DWORD WINAPI redirected_temp_a(DWORD capacity, LPSTR output) noexcept {
  if (!active_paths || !active_paths->isolation)
    return GetTempPathA(capacity, output);
  if (active_paths->temp_ansi.empty()) {
    SetLastError(ERROR_NO_UNICODE_TRANSLATION);
    return 0;
  }
  return copy_path<char>(active_paths->temp_ansi, capacity, output);
}
DWORD WINAPI redirected_temp_w(DWORD capacity, LPWSTR output) noexcept {
  if (!active_paths || !active_paths->isolation)
    return GetTempPathW(capacity, output);
  return copy_path<wchar_t>(active_paths->temp_wide, capacity, output);
}
PatchResult install_folder_redirects(const RedirectPaths &paths) noexcept {
  active_paths = &paths;
  const std::array entries{
      ImportReplacement{"SHGetKnownFolderPath", reinterpret_cast<void *>(&redirected_known_folder),
                        reinterpret_cast<void *>(&SHGetKnownFolderPath)},
      ImportReplacement{"GetTempPathA", reinterpret_cast<void *>(&redirected_temp_a),
                        reinterpret_cast<void *>(&GetTempPathA)},
      ImportReplacement{"GetTempPathW", reinterpret_cast<void *>(&redirected_temp_w),
                        reinterpret_cast<void *>(&GetTempPathW)}};
  return patch_main_imports(entries);
}

HRESULT install_local_folder_hook(const RedirectPaths &paths) noexcept {
  try {
    if (!paths.isolation || paths.local_folder.empty())
      return E_INVALIDARG;
    std::filesystem::create_directories(paths.local_folder);

    const HRESULT com = RoInitialize(RO_INIT_MULTITHREADED);
    if (FAILED(com) && com != RPC_E_CHANGED_MODE)
      return com;

    using Microsoft::WRL::ComPtr;
    using Microsoft::WRL::Wrappers::HStringReference;
    ComPtr<ABI::Windows::Storage::IApplicationDataStatics> statics;
    HRESULT hr = Windows::Foundation::GetActivationFactory(
        HStringReference(RuntimeClass_Windows_Storage_ApplicationData).Get(), &statics);
    if (FAILED(hr))
      return hr;
    ComPtr<ABI::Windows::Storage::IApplicationData> data;
    hr = statics->get_Current(&data);
    if (FAILED(hr))
      return hr;

    active_paths = &paths;
    replace_vtable_slot(data.Get(), get_local_folder_index,
                        reinterpret_cast<void *>(&redirected_folder),
                        reinterpret_cast<void **>(&original_get_local_folder));
    replace_vtable_slot(data.Get(), get_temporary_folder_index,
                        reinterpret_cast<void *>(&redirected_folder), nullptr);
    ComPtr<ABI::Windows::Storage::IApplicationData2> data2;
    if (SUCCEEDED(data.As(&data2)))
      replace_vtable_slot(data2.Get(), get_local_cache_folder_index,
                          reinterpret_cast<void *>(&redirected_folder), nullptr);
    return S_OK;
  } catch (const std::exception &e) {
    log(LogLevel::error, "LocalFolder redirect failed: {}", e.what());
    return E_FAIL;
  } catch (...) {
    write_log(LogLevel::error, "LocalFolder redirect failed");
    return E_FAIL;
  }
}
} // namespace levi
