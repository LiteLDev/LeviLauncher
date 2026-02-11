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
    "app.lang",
    "common.deleting",
    "common.error_load",
    "common.unknown",
    "common.update_all",
    "common.updated",
    "downloadpage.delete.confirm",
    "downloadpage.table.header.details",
    "downloadpage.table.header.disable",
    "downloadpage.table.header.enable",
    "downloadpage.table.header.open_folder",
    "downloadpage.table.header.remove",
    "downloadpage.table.header.update",
    "downloadpage.table.header.update_all",
    "downloadpage.table.header.updated",
    "launcherpage.adminconfirm.cancel_button",
    "launcherpage.adminconfirm.confirm_button",
    "launcherpage.adminconfirm.content",
    "launcherpage.adminconfirm.title",
    "launcherpage.admindeny.cancel_button",
    "launcherpage.admindeny.confirm_button",
    "launcherpage.admindeny.content",
    "launcherpage.admindeny.title",
    "launcherpage.delete.confirm.cancel_button",
    "launcherpage.delete.confirm.delete_button",
    "mods.delete_summary_title_done",
    "mods.delete_summary_title_failed",
    "mods.summary_deleted",
    "settings.body.appearance.disable_animations",
    "settings.body.appearance.disable_animations_desc",
    "settings.body.appearance.title",
    "settings.layout.desc",
    "settings.layout.title",
    "versions.edit.danger_zone_title",
    "versions.info.version",

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
