import json
import os
from collections import OrderedDict

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
    "about.check_update",
    "about.curseforge",
    "about.developer",
    "about.github",
    "about.mcbbs",
    "about.modrinth",
    "about.publish",
    "about.qq",
    "about.version",
    "download_manager.short_title",
    "downloadpage.delete.success_title",
    "downloadpage.download.success_title",
    "downloadpage.mirror.latency",
    "downloadpage.progress.title",
    "downloadpage.topcontent.ll_supported",
    "downloadpage.topcontent.ll_unsupported",
    "filemanager.choose_drive",
    "filemanager.choose_this_dir",
    "filemanager.computer",
    "filemanager.dir_not_writable",
    "filemanager.drives_title",
    "filemanager.new_folder",
    "filemanager.pin_current",
    "filemanager.pinned",
    "filemanager.quick_access",
    "filemanager.refreshing",
    "filemanager.search_placeholder",
    "filemanager.sort.kind",
    "filemanager.up",
    "settings.lip.error.create_target_dir",
    "settings.lip.error.create_temp",
    "settings.lip.error.download_failed",
    "settings.lip.error.fetch_release_failed",
    "settings.lip.error.unzip_failed",
    "settings.lip.status.checking_update",
    "settings.lip.status.done",
    "settings.lip.status.downloading",
    "settings.lip.status.extracting",
    "settings.process.launcher_version",
    "settings.process.path",
    "settings.process.pid",
    "settings.unsaved.cancelled"
]

locales_dir = r'd:\a\LiteLDev\LeviLauncher\frontend\src\assets\locales'
locale_files = ['en_US.json', 'ru_RU.json', 'zh_CN.json', 'ja_JP.json', 'zh_HK.json']

for lf in locale_files:
    path = os.path.join(locales_dir, lf)
    if os.path.exists(path):
        with open(path, 'r', encoding='utf-8') as f:
            data = json.load(f, object_pairs_hook=OrderedDict)
        
        original_count = len(json.dumps(data))
        cleaned_data = remove_keys(data, set(unused_keys))
        new_count = len(json.dumps(cleaned_data))
        
        with open(path, 'w', encoding='utf-8') as f:
            json.dump(cleaned_data, f, ensure_ascii=False, indent=2)
        
        print(f'Processed {lf}: reduced size from {original_count} to {new_count} characters.')
