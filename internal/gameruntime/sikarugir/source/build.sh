#!/bin/bash
set -euo pipefail
cd "$(dirname "$0")"
compiler="${CC:-x86_64-w64-mingw32-gcc}"
flags=(-O1 -Wall -Wno-unused-function -shared -static -static-libgcc -Wl,--kill-at -Wl,--no-insert-timestamp -s)
"$compiler" "${flags[@]}" -o ../xgameruntime.dll xgameruntime/xgameruntime.c xgameruntime/system.c xgameruntime/taskqueue.c xgameruntime/extra_*.c -lwinhttp -lbcrypt
"$compiler" "${flags[@]}" -o ../mcwinrt.dll winrt/mcwinrt.c -lole32
shasum -a 256 ../xgameruntime.dll ../mcwinrt.dll
