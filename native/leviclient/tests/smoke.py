#!/usr/bin/env python3
"""Exercise the built client through Sikarugir, without game files or real accounts."""
import argparse
import json
import os
from pathlib import Path
import shlex
import shutil
import subprocess
import threading
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

p = argparse.ArgumentParser()
p.add_argument('--compiler', required=True, help='zig cc -target x86_64-windows-gnu')
p.add_argument('--launcher', required=True, type=Path, help='Sikarugir Contents/MacOS/wineskinlauncher')
p.add_argument('--output', required=True, type=Path)
p.add_argument('--client', type=Path, help='Alternate client artifact for regression comparison')
args = p.parse_args()
root = Path(__file__).resolve().parents[3]
source = Path(__file__).resolve().parent
output = args.output.resolve()
output.mkdir(parents=True, exist_ok=True)
cc = shlex.split(args.compiler) + ['-O2']
subprocess.run(cc + ['-shared', str(source/'fixture_api.c'), str(source/'fixture-api.def'), '-o', str(output/'fixture-api.dll')], check=True)
subprocess.run(cc + ['-shared', '-fno-builtin', str(source/'fixture_runtime.c'), str(output/'fixture_api.lib'), '-o', str(output/'xgameruntime.dll')], check=True)
subprocess.run(cc + ['-shared', str(source/'fixture_preloader.c'), '-o', str(output/'PreLoader.dll')], check=True)
subprocess.run(cc + [str(source/'fixture_main.c'), '-o', str(output/'fixture.exe')], check=True)
shutil.copyfile(args.client or root/'internal/gameclient/LeviClient.dll', output/'LeviClient.dll')

mode, received = 'accepted', []
class Handler(BaseHTTPRequestHandler):
    def log_message(self, *unused): pass
    def do_POST(self):
        body = json.loads(self.rfile.read(int(self.headers['Content-Length'])))
        assert self.headers['Authorization'] == 'Bearer fixture-token'
        received.append((self.path, body))
        self.send_response(200 if mode == 'accepted' else 503)
        self.send_header('Content-Length', '0')
        self.end_headers()

server = ThreadingHTTPServer(('127.0.0.1', 0), Handler)
threading.Thread(target=server.serve_forever, daemon=True).start()
report = {}
try:
    for mode, expected in [('accepted', 1), ('fallback', 2), ('inert', 2)]:
        received.clear()
        result = output/'result.txt'
        result.unlink(missing_ok=True)
        env = {k: v for k, v in os.environ.items() if not k.startswith(('WINE', 'DYLD_')) and k not in ('LEVI_GAME_AUTH_URL', 'LEVI_GAME_AUTH_TOKEN')}
        if mode != 'inert':
            env.update(LEVI_GAME_AUTH_URL=f'http://127.0.0.1:{server.server_port}/v1/game-login/fixture', LEVI_GAME_AUTH_TOKEN='fixture-token')
        subprocess.run([str(args.launcher), str(output/'fixture.exe')], cwd=output, env=env, timeout=30, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, check=True)
        deadline = time.monotonic() + 30
        while not result.exists() and time.monotonic() < deadline:
            time.sleep(0.1)
        actual = result.read_text().strip()
        guard = -1 if mode == 'inert' else 1
        assert actual == f'prompt={expected} other=2 guard={guard}', (mode, actual)
        if mode == 'accepted':
            assert received == [('/v1/game-login/fixture', {'code': 'AB12CD34', 'expiresIn': 900, 'resultEvents': True}), ('/v1/game-login/fixture/result', {'phase': 'failed', 'errorCode': '80004005'})], received
        elif mode == 'inert':
            assert not received, received
        report[mode] = {'passed': True, 'promptResult': expected, 'unrelatedPromptResult': 2, 'heapGuard': guard == 1 if mode != 'inert' else 'inert'}
        print(f'{mode}: passed', flush=True)
finally:
    server.shutdown()
(output/'report.json').write_text(json.dumps(report, indent=2)+'\n')
