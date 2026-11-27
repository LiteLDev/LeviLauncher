# Minecraft compatibility shims for Wine

The C sources and x64 Windows DLLs are based on
[Flarial's macOS fork](https://github.com/flarialmc/macos/tree/fd7c8991ca151bfaef60882152e8b270bca68daa)
at commit `fd7c8991ca151bfaef60882152e8b270bca68daa`, under GPL version 3.
The launcher builds modified `xgameruntime.dll` and `mcwinrt.dll` shims with trace-only logging, `LEVI_WINE_WORKDIR` working-directory restoration, event-driven task-queue termination and reference-counted queue lifetimes. Run `bash build.sh` with MinGW-w64 to rebuild the DLLs. Set `XGR_TRACE=1` (or `MCWINRT_TRACE=1`) to debug calls. Each traced process starts a fresh `C:\xgr.log` / `C:\mcwinrt.log`.
The license text is in the launcher's `COPYING` file.

| Shipped DLL | SHA-256 |
| --- | --- |
| `xgameruntime.dll` | `ab508c0fa55c85e5ec809ab0413e0234066d5f1642a5d597f746459922c554c2` |
| `mcwinrt.dll` | `8e9d3f9e290f5833c30e78831c6702524a1a5d8f408da7c67c382719420da3f0` |
