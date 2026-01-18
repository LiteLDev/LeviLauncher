import json
import os

def get_keys(d, prefix=''):
    keys = []
    if not isinstance(d, dict):
        return []
    for k, v in d.items():
        new_prefix = f'{prefix}.{k}' if prefix else k
        if isinstance(v, dict):
            keys.extend(get_keys(v, new_prefix))
        else:
            keys.append(new_prefix)
    return keys

locales_dir = r'd:\a\LiteLDev\LeviLauncher\frontend\src\assets\locales'
locale_files = ['en_US.json', 'ru_RU.json', 'zh_CN.json']

all_keys = set()
for lf in locale_files:
    path = os.path.join(locales_dir, lf)
    if os.path.exists(path):
        with open(path, encoding='utf-8') as f:
            data = json.load(f)
            all_keys.update(get_keys(data))

used_keys = set()
# Always keep errors group and some other potentially dynamic groups
keep_prefixes = ['errors.', 'file.types.', 'curseforge.sort.']
for k in all_keys:
    for prefix in keep_prefixes:
        if k.startswith(prefix):
            used_keys.add(k)

search_dirs = [r'd:\a\LiteLDev\LeviLauncher\frontend\src', r'd:\a\LiteLDev\LeviLauncher\internal']
extensions = ('.ts', '.tsx', '.go', '.js', '.jsx', '.html')

for sdir in search_dirs:
    for root, dirs, files in os.walk(sdir):
        if 'node_modules' in dirs:
            dirs.remove('node_modules')
        for file in files:
            if file.endswith(extensions):
                path = os.path.join(root, file)
                with open(path, 'r', encoding='utf-8', errors='ignore') as f:
                    content = f.read()
                    for key in list(all_keys - used_keys):
                        if f'"{key}"' in content or f"'{key}'" in content or f'`{key}`' in content:
                            used_keys.add(key)

unused_keys = all_keys - used_keys
print(f'Unused keys found: {len(unused_keys)}')
for k in sorted(unused_keys):
    print(k)
