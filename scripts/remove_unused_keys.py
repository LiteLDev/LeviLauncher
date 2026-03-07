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
    "common.all",
    "common.delete_selected",
    "contentpage.transfer_resources_body",
    "contentpage.update_material_bin_stat_skipped",
    "lip.files.action_success",
    "lip.files.compatibility.compatible",
    "lip.files.compatibility.incompatible",
    "lip.files.compatibility.no_ll_requirement",
    "lip.files.compatibility.requires_ll",
    "lip.files.compatibility_label",
    "lip.files.confirm_install_body",
    "lip.files.confirm_install_title",
    "lip.files.confirm_uninstall_body",
    "lip.files.confirm_uninstall_title",
    "lip.files.fixed_target_instance_label",
    "lip.files.installed_version",
    "lip.files.instance_ll_version",
    "lip.files.mapping_unavailable_hint",
    "lip.files.only_compatible",
    "lip.files.select_instance_placeholder",
    "lip.files.select_version_placeholder",
    "lip.files.select_version_title",
    "lip.files.target_instance_label",
    "lip.files.uninstall",
    "lip.files.update",
    "lip.ll_filter_context",
    "lip.maintenance",
    "lip.task_console.method_label",
    "lip.task_console.stage_install_ll",
    "mods.confirm_delete_title",
    "settings.lip.description",
    "settings.lip.installed",
    "settings.lip.installed_label",
    "settings.lip.installed_without_version",
    "settings.lip.path_label",
    "settings.lip.repair_button",
    "versions.edit.loader.ll_current_version",
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
