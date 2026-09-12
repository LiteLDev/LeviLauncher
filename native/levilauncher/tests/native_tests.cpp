// SPDX-License-Identifier: GPL-3.0-only
#include "config/mod_loader.h"
#include "config/version_config.h"
#include "hook/iat_hook.h"
#include "utils/windows_paths.h"
#include <Windows.h>
#include <array>
#include <cstring>
#include <filesystem>
#include <fstream>
#include <iostream>
#include <sstream>
#include <stdexcept>
#include <vector>

namespace fs = std::filesystem;
namespace {
void check(bool value, const char *message) {
  if (!value)
    throw std::runtime_error(message);
}
template <typename Fn> void rejects(Fn fn, const char *message) {
  try {
    fn();
  } catch (const std::exception &) {
    return;
  }
  throw std::runtime_error(message);
}
levi::VersionConfig config(const char *text) {
  std::istringstream input(text);
  return levi::parse_version_config(input);
}
void write(const fs::path &path, const char *text) {
  fs::create_directories(path.parent_path());
  std::ofstream(path, std::ios::binary) << text;
}
void test_config() {
  check(config("{}").isolation, "missing flag defaults to isolation");
  check(!config(R"({"enableIsolation":false})").isolation, "boolean false");
  check(!config(R"({"enableIsolation":"FaLsE"})").isolation, "legacy string false");
  check(!config(R"({"enableIsolation":0})").isolation, "legacy numeric false");
  check(config(R"({"note":"enableIsolation: false","nested":{"enableIsolation":false}})").isolation,
        "unrelated text must not change isolation");
  rejects([] { config("{"); }, "malformed JSON");
  rejects([] { config("[]"); }, "non-object JSON");
  const fs::path game = L"C:\\Game";
  check(levi::legacy_data_directory(game, {}) == game, "missing metadata");
  check(levi::legacy_data_directory(game, config(R"({"gameVersion":"1.21.100"})")) ==
            game / L"Minecraft Bedrock",
        "legacy release");
  check(levi::legacy_data_directory(game, config(R"({"gameVersion":"1.25.0","type":"preview"})")) ==
            game / L"Minecraft Bedrock Preview",
        "legacy preview");
  check(levi::legacy_data_directory(game, config(R"({"gameVersion":"1.26.0.24"})")) ==
            game / L"Minecraft Bedrock Preview",
        "special preview version");
  check(levi::legacy_data_directory(game, config(R"({"gameVersion":"1.26.0.0"})")) == game,
        "new version");
}
void test_strings() {
  const std::wstring unicode = L"测试\\Mods\\\U0001f600";
  check(levi::from_utf8(levi::to_utf8(unicode)) == unicode, "Unicode roundtrip");
  rejects([] { levi::from_utf8("\xc0\xaf"); }, "reject invalid UTF-8");
  const std::wstring path = L"C:\\TEMP\\";
  check(levi::copy_path<wchar_t>(path, 0, nullptr) == path.size() + 1, "size query includes NUL");
  std::array<wchar_t, 32> output;
  output.fill(L'!');
  check(levi::copy_path<wchar_t>(path, 2, output.data()) == path.size() + 1, "short buffer size");
  check(output[0] == L'!', "short buffer must not be truncated");
  check(levi::copy_path<wchar_t>(path, static_cast<DWORD>(output.size()), output.data()) ==
            path.size(),
        "successful length excludes NUL");
  check(path == output.data(), "exact wide path");
  check(levi::copy_path<wchar_t>(path, 32, nullptr) == 0 &&
            GetLastError() == ERROR_INVALID_PARAMETER,
        "invalid output pointer");
  std::array<char, 16> ansi{};
  check(levi::copy_path<char>("abc\\", 5, ansi.data()) == 4 &&
            std::strcmp(ansi.data(), "abc\\") == 0,
        "ANSI exact capacity");
}
void test_imports() {
  constexpr std::size_t size = 4096;
  auto *memory = static_cast<std::byte *>(
      VirtualAlloc(nullptr, size, MEM_COMMIT | MEM_RESERVE, PAGE_READWRITE));
  check(memory != nullptr, "allocate image");
  struct Cleanup {
    void *p;
    ~Cleanup() { VirtualFree(p, 0, MEM_RELEASE); }
  } cleanup{memory};
  std::span image(memory, size);
  auto *dos = reinterpret_cast<IMAGE_DOS_HEADER *>(memory);
  dos->e_magic = IMAGE_DOS_SIGNATURE;
  dos->e_lfanew = 128;
  auto *nt = reinterpret_cast<IMAGE_NT_HEADERS64 *>(memory + 128);
  nt->Signature = IMAGE_NT_SIGNATURE;
  nt->FileHeader.SizeOfOptionalHeader = sizeof(IMAGE_OPTIONAL_HEADER64);
  nt->OptionalHeader.Magic = IMAGE_NT_OPTIONAL_HDR64_MAGIC;
  nt->OptionalHeader.SizeOfImage = size;
  nt->OptionalHeader.NumberOfRvaAndSizes = IMAGE_NUMBEROF_DIRECTORY_ENTRIES;
  nt->OptionalHeader.DataDirectory[IMAGE_DIRECTORY_ENTRY_IMPORT] = {512, 60};
  auto *descriptors = reinterpret_cast<IMAGE_IMPORT_DESCRIPTOR *>(memory + 512);
  std::memcpy(memory + 700, "kernel32.dll", 13);
  std::memcpy(memory + 800 + 2, "GetTempPathW", 13);
  for (int i = 0; i < 2; ++i) {
    descriptors[i].Name = 700;
    descriptors[i].FirstThunk = 1024 + i * 64;
    descriptors[i].OriginalFirstThunk = 1280 + i * 64;
    auto *lookup =
        reinterpret_cast<IMAGE_THUNK_DATA64 *>(memory + descriptors[i].OriginalFirstThunk);
    lookup[0].u1.AddressOfData = 800;
    auto *slot = reinterpret_cast<IMAGE_THUNK_DATA64 *>(memory + descriptors[i].FirstThunk);
    slot[0].u1.Function = 0x1234;
  }
  const std::array replacement{levi::ImportReplacement{
      "GetTempPathW", reinterpret_cast<void *>(0x5678), reinterpret_cast<void *>(0x1234)}};
  auto result = levi::patch_imports(image, replacement);
  check(result.valid_image && result.error == 0 && result.patched == 2,
        "patch all matching descriptors");
  check(levi::patch_imports(image, replacement).patched == 0, "idempotent hook");
  auto *slot = reinterpret_cast<IMAGE_THUNK_DATA64 *>(memory + 1024);
  slot->u1.Function = 0x1234;
  descriptors[0].OriginalFirstThunk = 0;
  check(levi::patch_imports(image, replacement).patched == 1,
        "absent lookup table uses resolved address");
  descriptors[0].FirstThunk = 4090;
  check(!levi::patch_imports(image, replacement).valid_image, "reject out of bounds IAT");
  check(!levi::patch_imports(image.first(100), replacement).valid_image,
        "reject truncated headers");
}
void test_mods(const fs::path &root, const fs::path &binaries) {
  const auto mod = root / L"mods" / L"测试 Mod";
  fs::create_directories(mod / L"bin");
  fs::copy_file(binaries / L"native_fixture.dll", mod / L"bin" / L"native_fixture.dll");
  write(mod / L"manifest.json",
        R"({"type":"preload-native","name":"测试","entry":"bin/native_fixture.dll"})");
  write(root / L"mods" / L"bad-json" / L"manifest.json", "{");
  write(root / L"mods" / L"bad-types" / L"manifest.json",
        R"({"type":"preload-native","entry":12})");
  check(levi::resolve_mod_entry(mod, "bin/native_fixture.dll") ==
            fs::canonical(mod / L"bin/native_fixture.dll"),
        "valid Mod entry");
  write(root / L"outside.dll", "fixture");
  rejects([&] { levi::resolve_mod_entry(mod, "../../outside.dll"); }, "reject path escape");
  rejects([&] { levi::resolve_mod_entry(mod, "C:/outside.dll"); }, "reject absolute path");
  rejects([&] { levi::resolve_mod_entry(mod, "bin/native_fixture.dll:stream"); },
          "reject alternate data stream");
  rejects([&] { levi::resolve_mod_entry(mod, std::string("bad\0.dll", 8)); },
          "reject embedded NUL");
  const auto result = levi::load_native_mods(root);
  check(result.loaded == 1 && result.failed == 2, "invalid manifest must not block later Mods");
  check(GetModuleHandleW(L"native_fixture.dll") != nullptr, "DLL actually loaded");
  write(root / L"preloader.dll", "external preloader marker");
  const auto skipped = levi::load_native_mods(root);
  check(skipped.external_preloader && skipped.loaded == 0 && skipped.failed == 0,
        "external preloader owns Mods");
}
void smoke(const fs::path &directory, const fs::path &binaries, const char *metadata,
           const wchar_t *data_subdirectory) {
  fs::create_directories(directory);
  fs::copy_file(binaries / L"Minecraft.Windows.exe", directory / L"Minecraft.Windows.exe");
  fs::copy_file(binaries / L"LeviLauncher.dll", directory / L"LeviLauncher.dll");
  if (metadata)
    write(directory / L"version.json", metadata);
  write(directory / L"mods" / L"test" / L"manifest.json",
        R"({"type":"preload-native","entry":"native_fixture.dll"})");
  fs::copy_file(binaries / L"native_fixture.dll",
                directory / L"mods" / L"test" / L"native_fixture.dll");
  auto command = L"\"" + (directory / L"Minecraft.Windows.exe").wstring() + L"\" " +
                 (data_subdirectory ? L"on" : L"off") + L" \"" +
                 (data_subdirectory ? data_subdirectory : L"") + L"\"";
  STARTUPINFOW startup{};
  startup.cb = sizeof(startup);
  PROCESS_INFORMATION process{};
  check(CreateProcessW(nullptr, command.data(), nullptr, nullptr, FALSE, CREATE_NO_WINDOW, nullptr,
                       directory.c_str(), &startup, &process) != FALSE,
        "start smoke host");
  CloseHandle(process.hThread);
  const DWORD wait = WaitForSingleObject(process.hProcess, 15000);
  if (wait != WAIT_OBJECT_0) {
    TerminateProcess(process.hProcess, 99);
    WaitForSingleObject(process.hProcess, 5000);
  }
  DWORD code = 99;
  GetExitCodeProcess(process.hProcess, &code);
  CloseHandle(process.hProcess);
  check(wait == WAIT_OBJECT_0 && code == 0, "DLL attach, folder hooks and async Mod smoke");
}
} // namespace

int main() {
  fs::path scratch;
  try {
    test_config();
    test_strings();
    test_imports();
    const auto binaries = levi::module_path(nullptr).parent_path();
    scratch =
        fs::temp_directory_path() / (L"LeviNativeTests-" + std::to_wstring(GetCurrentProcessId()));
    fs::create_directory(scratch);
    smoke(scratch / L"隔离 on", binaries, R"({"enableIsolation":true,"gameVersion":"1.26.0"})",
          L"");
    smoke(scratch / L"隔离 off", binaries, R"({"enableIsolation":false})", nullptr);
    smoke(scratch / L"旧版 release", binaries, R"({"gameVersion":"1.21.100"})",
          L"Minecraft Bedrock");
    smoke(scratch / L"旧版 preview", binaries, R"({"gameVersion":"1.25.0","type":"preview"})",
          L"Minecraft Bedrock Preview");
    smoke(scratch / L"特殊 preview", binaries, R"({"gameVersion":"1.26.0.24"})",
          L"Minecraft Bedrock Preview");
    smoke(scratch / L"缺少 metadata", binaries, nullptr, L"");
    test_mods(scratch / L"mods-unit", binaries);
    // Release only the test fixture so Windows allows removal of the scratch tree.
    if (auto module = GetModuleHandleW(L"native_fixture.dll"))
      FreeLibrary(module);
    fs::remove_all(scratch);
    std::cout << "All native tests passed\n";
    return 0;
  } catch (const std::exception &e) {
    std::cerr << "Native tests failed: " << e.what() << '\n';
    return 1;
  }
}
