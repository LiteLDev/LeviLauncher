import json
from collections import OrderedDict
from pathlib import Path

def remove_keys(data, keys_to_remove, prefix=''):
    if not isinstance(data, dict):
        return data
    
    new_data = OrderedDict()
    for k, v in data.items():
        full_key = f'{prefix}.{k}' if prefix else k
        if full_key in keys_to_remove:
            continue
        
        if isinstance(v, dict):
            cleaned_v = remove_keys(v, keys_to_remove, full_key)
            if cleaned_v: # Only keep if not empty
                new_data[k] = cleaned_v
        else:
            new_data[k] = v
    return new_data

# The list of unused keys from previous run
unused_keys = [
    "common.update",
    "launcherpage.gdk_missing.body",
    "launcherpage.gdk_missing.go_settings",
    "launcherpage.gdk_missing.title",
    "lip.files.current_installed_version",
    "lip.files.instance_game_version",
    "lip.files.ll_missing_auto_install_hint",
    "lip.files.ll_missing_mapping_block_hint",
    "lip.files.ll_missing_no_compatible_hint",
    "mods.err_lip_not_installed",
    "settings.gdk.download.title",
    "settings.gdk.download_unavailable",
    "settings.gdk.install.body",
    "settings.gdk.install.title",
    "settings.gdk.install_button",
    "settings.gdk.installed",
    "settings.gdk.license.accept",
    "settings.gdk.license.body",
    "settings.gdk.license.link_text",
    "settings.gdk.license.title",
    "settings.gdk.path_label",
    "settings.gdk.title",
    "versions.edit.backup.restore.pack_uuid_hint",
    "versions.edit.backup.restore.section_body",
    "versions.edit.loader.ll_only_latest_when_current_unknown",
    "versions.edit.loader.ll_upgrade_only_hint",
]

ROOT_DIR = Path(__file__).resolve().parents[1]
LOCALES_DIR = ROOT_DIR / "frontend" / "src" / "assets" / "locales"
LOCALE_FILES = sorted(LOCALES_DIR.glob("*.json"))

if not LOCALE_FILES:
    raise FileNotFoundError(f"No locale files found in {LOCALES_DIR}")

for locale_file in LOCALE_FILES:
    if locale_file.exists():
        with locale_file.open('r', encoding='utf-8') as f:
            data = json.load(f, object_pairs_hook=OrderedDict)
        
        original_count = len(json.dumps(data))
        cleaned_data = remove_keys(data, set(unused_keys))
        new_count = len(json.dumps(cleaned_data))
        
        with locale_file.open('w', encoding='utf-8') as f:
            json.dump(cleaned_data, f, ensure_ascii=False, indent=2)
        
        print(f'Processed {locale_file.name}: reduced size from {original_count} to {new_count} characters.')
