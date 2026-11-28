#!/usr/bin/env python3
"""Build the native Wails macOS app. Wine remains an external game runtime."""
import argparse, hashlib, os, pathlib, plistlib, re, shutil, subprocess, tempfile
root = pathlib.Path(__file__).resolve().parents[2]


def read_app_version(config_path: pathlib.Path, default: str = '1.1.0') -> str:
    if not config_path.is_file():
        return default
    in_info = False
    for line in config_path.read_text(encoding='utf-8').splitlines():
        if re.match(r'^info:\s*$', line):
            in_info = True
            continue
        if in_info and line and not line[0].isspace():
            break
        if in_info:
            match = re.match(r'^\s+version:\s*"([^"]+)"', line)
            if match:
                return match.group(1)
    return default
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
app_version = read_app_version(root/'build/config.yml')
info={'CFBundleExecutable':'LeviLauncher','CFBundleIdentifier':'org.levimc.levilauncher.native','CFBundleName':'LeviLauncher','CFBundleDisplayName':'LeviLauncher','CFBundlePackageType':'APPL','CFBundleShortVersionString':app_version,'CFBundleVersion':'1','CFBundleIconFile':'AppIcon.icns','LSMinimumSystemVersion':'14.6','NSHighResolutionCapable':True,'NSAppTransportSecurity':{'NSAllowsLocalNetworking':True},'NSSupportsAutomaticGraphicsSwitching':True,'CFBundleURLTypes':[{'CFBundleURLName':'org.levimc.levilauncher.native','CFBundleURLSchemes':['levilauncher'],'CFBundleTypeRole':'Viewer'}]}
document_types = {
    'mcpack': 'Minecraft Pack',
    'mcaddon': 'Minecraft Add-On',
    'mcworld': 'Minecraft World',
    'mcmod': 'Levi Mod Package',
}
info['CFBundleDocumentTypes'] = [
    {
        'CFBundleTypeName': name,
        'CFBundleTypeRole': 'Editor',
        'LSHandlerRank': 'Owner' if ext == 'mcmod' else 'Alternate',
        'LSItemContentTypes': [f'org.levimc.levilauncher.{ext}'],
    }
    for ext, name in document_types.items()
]
for ext, name in document_types.items():
    key = 'UTExportedTypeDeclarations' if ext == 'mcmod' else 'UTImportedTypeDeclarations'
    info.setdefault(key, []).append({
        'UTTypeIdentifier': f'org.levimc.levilauncher.{ext}',
        'UTTypeDescription': name,
        'UTTypeConformsTo': ['public.data', 'public.content'],
        'UTTypeTagSpecification': {'public.filename-extension': [ext]},
    })
with (app/'Contents/Info.plist').open('wb') as f: plistlib.dump(info,f)
for name in ('COPYING', 'THIRD_PARTY_NOTICES'):
    shutil.copyfile(root/name, resources/name)

# Build the native macOS MetalFX helper dylib and Game Mode host app bundle.
metal_helper_src = root/'native/macos/metal_spatial_upscale.m'
game_host_src = root/'native/macos/levi_game_host.c'
metal_helper_out = resources/'libLeviMetalFX.dylib'
subprocess.run([
    'clang', '-arch', 'x86_64', '-mmacosx-version-min=14.6', '-O2', '-fobjc-arc',
    '-Wall', '-Wextra', '-Werror', '-dynamiclib', str(metal_helper_src),
    '-framework', 'Foundation', '-weak_framework', 'AppKit',
    '-weak_framework', 'QuartzCore', '-weak_framework', 'Metal',
    '-weak_framework', 'MetalFX', '-framework', 'IOKit', '-o', str(metal_helper_out),
], check=True)
subprocess.run(['codesign', '--force', '--sign', '-', str(metal_helper_out)], check=True)

game_host_app = resources/'LeviGameHost.app'
if game_host_app.exists():
    shutil.rmtree(game_host_app)
(game_host_app/'Contents/MacOS').mkdir(parents=True, exist_ok=True)
(game_host_app/'Contents/Resources').mkdir(parents=True, exist_ok=True)
subprocess.run([
    'clang', '-arch', 'x86_64', '-mmacosx-version-min=14.6', '-O2',
    '-Wall', '-Wextra', '-Werror', str(game_host_src), '-framework', 'IOKit', '-framework', 'CoreFoundation',
    '-o', str(game_host_app/'Contents/MacOS/LeviGameHost'),
], check=True)
shutil.copyfile(resources/'AppIcon.icns', game_host_app/'Contents/Resources/AppIcon.icns')
host_info = {
    'CFBundleExecutable': 'LeviGameHost',
    'CFBundleIdentifier': 'org.levimc.levilauncher.gamehost',
    'CFBundleName': 'Minecraft Bedrock',
    'CFBundleDisplayName': 'Minecraft Bedrock',
    'CFBundlePackageType': 'APPL',
    'CFBundleShortVersionString': app_version,
    'CFBundleVersion': '1',
    'CFBundleIconFile': 'AppIcon.icns',
    'LSMinimumSystemVersion': '14.6',
    'NSHighResolutionCapable': True,
    'NSSupportsAutomaticGraphicsSwitching': True,
    'LSApplicationCategoryType': 'public.app-category.games',
    'LSSupportsGameMode': True,
    'GCSupportsGameMode': True,
}
with (game_host_app/'Contents/Info.plist').open('wb') as f: plistlib.dump(host_info, f)
subprocess.run(['codesign', '--force', '--sign', '-', str(game_host_app)], check=True)

subprocess.run(['codesign','--force','--sign','-',str(app)],check=True)
subprocess.run(['codesign','--verify','--deep','--strict',str(app)],check=True)
print(f'Native application: {app}')
print(f'Windows loader SHA256: {hashlib.sha256(loader.read_bytes()).hexdigest()}')
