set_project("LeviLauncherNative")
set_version("1.0.0")
set_xmakever("3.0.0")
set_plat("windows")
set_arch("x64")
set_languages("c++20")
set_runtimes("MD")
add_rules("mode.debug", "mode.release")
add_requires("nlohmann_json v3.11.3")

option("deploy")
    set_default(false)
    set_showmenu(true)
    set_description("Copy the DLL into the launcher's Go embed directory")
option_end()

local common_files = {
    "src/config/version_config.cpp", "src/config/mod_loader.cpp",
    "src/core/logger.cpp", "src/hook/iat_hook.cpp",
    "src/hook/folder_redirect.cpp", "src/utils/windows_paths.cpp"
}

target("LeviLauncher")
    set_kind("shared")
    add_files("src/core/dllmain.cpp", "src/core/exports.cpp")
    add_files(common_files)
    add_packages("nlohmann_json")
    add_defines("WIN32_LEAN_AND_MEAN", "NOMINMAX", "UNICODE", "_UNICODE")
    add_cxxflags("/utf-8", {tools = "cl"})
    set_warnings("allextra", "error")
    add_syslinks("shell32", "ole32", "uuid")
    after_build(function (target)
        if has_config("deploy") then
            local destination = path.join(os.scriptdir(), "../../internal/leviloader/LeviLauncher.dll")
            assert(os.isdir(path.directory(destination)), "Launcher embed directory does not exist")
            os.cp(target:targetfile(), destination)
        end
    end)

target("native_fixture")
    set_kind("shared")
    set_default(false)
    add_files("tests/fixture.cpp")

target("native_smoke")
    set_kind("binary")
    set_default(false)
    set_filename("Minecraft.Windows.exe")
    add_files("tests/smoke.cpp")
    add_deps("LeviLauncher")
    add_defines("WIN32_LEAN_AND_MEAN", "NOMINMAX", "UNICODE", "_UNICODE")
    add_cxxflags("/utf-8", {tools = "cl"})
    add_syslinks("shell32", "ole32", "uuid")

target("native_tests")
    add_deps("native_smoke", "native_fixture", {inherit = false})
    set_kind("binary")
    set_default(false)
    add_files("tests/native_tests.cpp")
    add_files(common_files)
    add_includedirs("src")
    add_packages("nlohmann_json")
    add_defines("WIN32_LEAN_AND_MEAN", "NOMINMAX", "UNICODE", "_UNICODE")
    add_cxxflags("/utf-8", {tools = "cl"})
    set_warnings("allextra", "error")
    add_syslinks("shell32", "ole32", "uuid")
    add_tests("default")
