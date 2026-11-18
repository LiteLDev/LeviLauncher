#!/usr/bin/env python3
"""Build the small Windows x64 preload client with the standard MinGW toolchain."""
import os
import shlex
import shutil
from pathlib import Path
import subprocess
import tempfile

root = Path(__file__).resolve().parents[2]
source = Path(__file__).resolve().parent
output = root / 'internal/gameclient/LeviClient.dll'
output.parent.mkdir(parents=True, exist_ok=True)
compiler = shlex.split(os.environ.get('MINGW_CC', 'x86_64-w64-mingw32-gcc'))
with tempfile.TemporaryDirectory() as scratch:
    artifact = Path(scratch)/'LeviClient.dll'
    subprocess.run([*compiler, '-O2', '-Wall', '-Wextra', '-Werror',
                    '-shared', '-s', str(source/'client.c'),
                    '-lwinhttp', '-luser32', '-o', str(artifact)], check=True)
    shutil.copyfile(artifact, output)
print(output)
