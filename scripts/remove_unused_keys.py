import json
from collections import OrderedDict

from check_unused_keys import LOCALE_FILES, collect_used_keys, load_all_keys


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


if not LOCALE_FILES:
    raise FileNotFoundError("No locale files found")

all_keys = load_all_keys()
used_keys, _ = collect_used_keys(all_keys)
unused_keys = all_keys - used_keys

print(f'Unused keys to remove: {len(unused_keys)}')
for key in sorted(unused_keys):
    print(f'  {key}')

for locale_file in LOCALE_FILES:
    with locale_file.open('r', encoding='utf-8') as f:
        data = json.load(f, object_pairs_hook=OrderedDict)

    original_count = len(json.dumps(data))
    cleaned_data = remove_keys(data, unused_keys)
    new_count = len(json.dumps(cleaned_data))

    with locale_file.open('w', encoding='utf-8') as f:
        json.dump(cleaned_data, f, ensure_ascii=False, indent=2)
        f.write('\n')

    print(f'Processed {locale_file.name}: reduced size from {original_count} to {new_count} characters.')
