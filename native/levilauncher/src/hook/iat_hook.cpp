// SPDX-License-Identifier: GPL-3.0-only
#include "iat_hook.h"
#include <cstring>
#include <limits>

namespace levi {
namespace {
template <typename T> T *at(std::span<std::byte> image, std::size_t offset) noexcept {
  if (offset > image.size() || sizeof(T) > image.size() - offset || offset % alignof(T) != 0)
    return nullptr;
  return reinterpret_cast<T *>(image.data() + offset);
}
std::string_view name_at(std::span<std::byte> image, std::size_t offset) noexcept {
  if (offset >= image.size())
    return {};
  const auto *text = reinterpret_cast<const char *>(image.data() + offset);
  const auto *end = static_cast<const char *>(std::memchr(text, 0, image.size() - offset));
  return end ? std::string_view(text, end) : std::string_view{};
}
bool is_system_import(std::string_view name) noexcept {
  const auto equals = [&](const char *value) { return _stricmp(name.data(), value) == 0; };
  return equals("kernel32.dll") || equals("kernelbase.dll") || equals("shell32.dll") ||
         _strnicmp(name.data(), "api-ms-win-core-file-", 21) == 0 ||
         _strnicmp(name.data(), "api-ms-win-shell-", 17) == 0;
}
} // namespace

PatchResult patch_imports(std::span<std::byte> image,
                          std::span<const ImportReplacement> replacements) noexcept {
  PatchResult result;
  const auto invalid = [&]() {
    result.valid_image = false;
    result.error = ERROR_BAD_EXE_FORMAT;
    return result;
  };
  const auto *dos = at<IMAGE_DOS_HEADER>(image, 0);
  if (!dos || dos->e_magic != IMAGE_DOS_SIGNATURE || dos->e_lfanew < 0)
    return invalid();
  const auto *nt = at<IMAGE_NT_HEADERS64>(image, static_cast<std::size_t>(dos->e_lfanew));
  if (!nt || nt->Signature != IMAGE_NT_SIGNATURE ||
      nt->OptionalHeader.Magic != IMAGE_NT_OPTIONAL_HDR64_MAGIC ||
      nt->FileHeader.SizeOfOptionalHeader < sizeof(IMAGE_OPTIONAL_HEADER64) ||
      nt->OptionalHeader.SizeOfImage > image.size() ||
      nt->OptionalHeader.NumberOfRvaAndSizes <= IMAGE_DIRECTORY_ENTRY_IMPORT)
    return invalid();
  const auto directory = nt->OptionalHeader.DataDirectory[IMAGE_DIRECTORY_ENTRY_IMPORT];
  if (!directory.VirtualAddress && !directory.Size)
    return result;
  if (!directory.VirtualAddress || directory.Size < sizeof(IMAGE_IMPORT_DESCRIPTOR) ||
      directory.VirtualAddress > image.size() ||
      directory.Size > image.size() - directory.VirtualAddress)
    return invalid();

  const auto descriptor_count = directory.Size / sizeof(IMAGE_IMPORT_DESCRIPTOR);
  for (std::size_t index = 0; index < descriptor_count; ++index) {
    const auto *descriptor = at<IMAGE_IMPORT_DESCRIPTOR>(
        image, directory.VirtualAddress + index * sizeof(IMAGE_IMPORT_DESCRIPTOR));
    if (!descriptor)
      return invalid();
    if (!descriptor->Name)
      return result;
    const auto dll = name_at(image, descriptor->Name);
    if (dll.empty())
      return invalid();
    if (!is_system_import(dll))
      continue;
    if (!descriptor->FirstThunk)
      return invalid();
    for (std::size_t thunk_index = 0;; ++thunk_index) {
      const auto offset = thunk_index * sizeof(IMAGE_THUNK_DATA64);
      auto *slot = at<IMAGE_THUNK_DATA64>(image, descriptor->FirstThunk + offset);
      if (!slot)
        return invalid();
      if (!slot->u1.Function)
        break;
      std::string_view function_name;
      if (descriptor->OriginalFirstThunk) {
        const auto *lookup = at<IMAGE_THUNK_DATA64>(image, descriptor->OriginalFirstThunk + offset);
        if (!lookup || !lookup->u1.AddressOfData)
          return invalid();
        if (IMAGE_SNAP_BY_ORDINAL64(lookup->u1.Ordinal))
          continue;
        if (lookup->u1.AddressOfData > std::numeric_limits<DWORD>::max())
          return invalid();
        function_name = name_at(image, static_cast<std::size_t>(lookup->u1.AddressOfData) +
                                           offsetof(IMAGE_IMPORT_BY_NAME, Name));
        if (function_name.empty())
          return invalid();
      }
      for (const auto &entry : replacements) {
        const auto current = reinterpret_cast<void *>(slot->u1.Function);
        const bool matches = descriptor->OriginalFirstThunk
                                 ? function_name == entry.name
                                 : entry.original && current == entry.original;
        if (!matches || !entry.replacement || current == entry.replacement)
          continue;
        DWORD old_protection = 0;
        if (!VirtualProtect(&slot->u1.Function, sizeof(void *), PAGE_READWRITE, &old_protection)) {
          result.error = GetLastError();
          return result;
        }
        InterlockedExchangePointer(reinterpret_cast<void *volatile *>(&slot->u1.Function),
                                   entry.replacement);
        ++result.patched;
        DWORD ignored = 0;
        if (!VirtualProtect(&slot->u1.Function, sizeof(void *), old_protection, &ignored)) {
          result.error = GetLastError();
          return result;
        }
        break;
      }
    }
  }
  return invalid(); // An import table must have a terminating descriptor.
}

PatchResult patch_main_imports(std::span<const ImportReplacement> replacements) noexcept {
  // This entry point accepts only the Windows loader's main image, never file bytes.
  auto *base = reinterpret_cast<std::byte *>(GetModuleHandleW(nullptr));
  if (!base)
    return {0, false, GetLastError()};
  const auto *dos = reinterpret_cast<const IMAGE_DOS_HEADER *>(base);
  const auto *nt = reinterpret_cast<const IMAGE_NT_HEADERS64 *>(base + dos->e_lfanew);
  return patch_imports({base, nt->OptionalHeader.SizeOfImage}, replacements);
}
} // namespace levi
