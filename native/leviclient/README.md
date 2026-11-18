# Levi game client

A small Windows x64 `preload-native` plugin for games launched by the native
macOS/Linux launcher. It forwards the existing Wine XGameRuntime device-code
prompt to Levi's loopback broker. Microsoft/Xbox authentication and credential
storage remain in XGameRuntime.

The broker supplies an ephemeral URL and bearer nonce through the game's
environment. The plugin changes only XGameRuntime's known account prompt; it
returns `IDOK` only after a successful local acknowledgement. Otherwise the
original dialog handles the request. Without these environment values it does
nothing, including on standard Windows launches. No launcher account tokens
are copied into the game.

The broker opens `https://www.microsoft.com/link?otc=…` with the system browser.
It sends state changes to Wails and exposes the latest state for a newly loaded
frontend. Per-process result events contain only success or an error code,
never the account identity. They avoid confusing two games sharing a prefix.

Build before the native launcher (the generated DLL is not committed):

```sh
python3 native/leviclient/build.py  # x86_64-w64-mingw32-gcc
# Alternatively:
MINGW_CC='zig cc -target x86_64-windows-gnu' python3 native/leviclient/build.py
```

Validation:

```sh
cc -Wall -Wextra -Werror native/leviclient/protocol_test.c -o /tmp/leviclient-protocol
/tmp/leviclient-protocol
go test -race ./internal/gameauth ./internal/gameclient
python3 native/leviclient/tests/smoke.py \
  --compiler 'zig cc -target x86_64-windows-gnu' \
  --launcher '/path/Minecraft Wine.app/Contents/MacOS/wineskinlauncher' \
  --output /tmp/leviclient-smoke
```

The Wine smoke test uses synthetic imported functions. It opens no dialogs,
touches no game data or shared status file, and uses no Microsoft account. It
checks acknowledgement, unavailable-broker fallback, unrelated dialog handling,
per-process result delivery, and inert behavior without a bridge environment.
Real account verification remains a separate in-game acceptance check.
