#!/usr/bin/env python3
"""Build the native Wails macOS app. Wine remains an external game runtime."""
import argparse, hashlib, os, pathlib, plistlib, shutil, subprocess, tempfile
root = pathlib.Path(__file__).resolve().parents[2]
p = argparse.ArgumentParser()
p.add_argument('--output', type=pathlib.Path, default=root/'bin/LeviLauncher.app')
p.add_argument('--skip-frontend', action='store_true')
p.add_argument('--loader', type=pathlib.Path, help='Windows x64 loader built from native/levilauncher')
args = p.parse_args()
loader = root/'internal/leviloader/LeviLauncher.dll'
if args.loader:
    if args.loader.resolve() != loader.resolve(): shutil.copyfile(args.loader, loader)
if not loader.is_file():
    p.error('Build the Windows x64 native loader first, or supply --loader with that artifact.')
env = dict(os.environ, MACOSX_DEPLOYMENT_TARGET='14.6', CGO_ENABLED='1')
env['CGO_CFLAGS'] = env.get('CGO_CFLAGS', '')+' -mmacosx-version-min=14.6'
env['CGO_LDFLAGS'] = env.get('CGO_LDFLAGS', '')+' -mmacosx-version-min=14.6'
if not args.skip_frontend:
    subprocess.run(['wails3', 'generate', 'bindings', '-f', '-tags production', '-d', str(root/'frontend/bindings')], cwd=root, env=env, check=True)
    subprocess.run(['npm', 'ci', '--no-audit', '--no-fund'], cwd=root/'frontend', check=True)
    subprocess.run(['npm', 'run', 'build'], cwd=root/'frontend', check=True)
app = args.output.resolve()
macos, resources = app/'Contents/MacOS', app/'Contents/Resources'
macos.mkdir(parents=True, exist_ok=True)
resources.mkdir(parents=True, exist_ok=True)
subprocess.run(['go', 'build', '-tags', 'production', '-trimpath', '-ldflags', '-extldflags=-mmacosx-version-min=14.6', '-o', str(macos/'LeviLauncher'), '.'], cwd=root, env=env, check=True)
with tempfile.TemporaryDirectory() as scratch:
    mac_icon = pathlib.Path(scratch)/'appicon-macos.png'
    subprocess.run(['swift', str(root/'build/native/icon.swift'), str(root/'build/appicon.png'), str(mac_icon)], check=True)
    iconset = pathlib.Path(scratch)/'AppIcon.iconset'
    iconset.mkdir()
    for size in [16, 32, 128, 256, 512]:
        for scale in [1, 2]:
            target=iconset/f'icon_{size}x{size}{"@2x" if scale==2 else ""}.png'
            subprocess.run(['sips', '-z', str(size*scale), str(size*scale), str(mac_icon), '--out', str(target)], stdout=subprocess.DEVNULL, check=True)
    subprocess.run(['iconutil', '-c', 'icns', str(iconset), '-o', str(resources/'AppIcon.icns')], check=True)
info={'CFBundleExecutable':'LeviLauncher','CFBundleIdentifier':'org.levimc.levilauncher.native','CFBundleName':'LeviLauncher','CFBundleDisplayName':'LeviLauncher','CFBundlePackageType':'APPL','CFBundleShortVersionString':'1.1.0','CFBundleVersion':'1','CFBundleIconFile':'AppIcon.icns','LSMinimumSystemVersion':'14.6','NSHighResolutionCapable':True,'NSAppTransportSecurity':{'NSAllowsLocalNetworking':True},'NSSupportsAutomaticGraphicsSwitching':True}
with (app/'Contents/Info.plist').open('wb') as f: plistlib.dump(info,f)
for name in ('COPYING', 'THIRD_PARTY_NOTICES'):
    shutil.copyfile(root/name, resources/name)
subprocess.run(['codesign','--force','--sign','-',str(app)],check=True)
subprocess.run(['codesign','--verify','--deep','--strict',str(app)],check=True)
print(f'Native application: {app}')
print(f'Windows loader SHA256: {hashlib.sha256(loader.read_bytes()).hexdigest()}')
