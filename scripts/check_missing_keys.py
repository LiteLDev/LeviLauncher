import json
import os
import re
from pathlib import Path

# Configuration
SRC_DIR = r'frontend/src'
LOCALES_DIR = r'frontend/src/assets/locales'

# Regex to match t('key') or t("key") or i18n.t('key')
KEY_PATTERN = re.compile(r'\b(?:t|i18n\.t)\s*\(\s*(["\'])([\w\.-]+)\1')

def load_json_keys(json_path):
    """Load and flatten keys from a JSON file."""
    if not os.path.exists(json_path):
        print(f"Error: Locale file not found at {json_path}")
        return set()
        
    with open(json_path, 'r', encoding='utf-8') as f:
        data = json.load(f)
        
    keys = set()
    
    def traverse(obj, prefix=''):
        if isinstance(obj, dict):
            for k, v in obj.items():
                full_key = f"{prefix}.{k}" if prefix else k
                if isinstance(v, (dict, list)):
                    traverse(v, full_key)
                else:
                    keys.add(full_key)
        elif isinstance(obj, list):
            pass
            
    traverse(data)
    return keys

def scan_code_for_keys(src_dir):
    """Scan source code for translation keys."""
    found_keys = set()
    
    for root, dirs, files in os.walk(src_dir):
        # Skip node_modules just in case
        if 'node_modules' in dirs:
            dirs.remove('node_modules')
            
        for file in files:
            if file.endswith(('.ts', '.tsx', '.js', '.jsx')):
                path = os.path.join(root, file)
                try:
                    with open(path, 'r', encoding='utf-8') as f:
                        content = f.read()
                        matches = KEY_PATTERN.findall(content)
                        for quote, key in matches:
                            found_keys.add(key)
                except Exception as e:
                    print(f"Error reading {path}: {e}")
                    
    return found_keys


def resolve_project_root():
    script_dir = Path(__file__).resolve().parent
    return script_dir.parent


def get_locale_files(locales_dir):
    return sorted(path.name for path in Path(locales_dir).glob("*.json"))

def main():
    project_root = str(resolve_project_root())
    src_path = os.path.join(project_root, SRC_DIR)
    locales_dir_path = os.path.join(project_root, LOCALES_DIR)
    locale_files = get_locale_files(locales_dir_path)
    
    print(f"Scanning code in: {src_path}")
    code_keys = scan_code_for_keys(src_path)
    print(f"Found {len(code_keys)} keys in code.\n")

    if not locale_files:
        print(f"Error: No locale files found in {locales_dir_path}")
        return
    
    print(f"Locale files scanned: {len(locale_files)}")
    print("")

    for locale_file in locale_files:
        locale_path = os.path.join(locales_dir_path, locale_file)
        print(f"Checking {locale_file}...")
        
        json_keys = load_json_keys(locale_path)
        if not json_keys:
            continue
            
        missing_keys = sorted(list(code_keys - json_keys))
        
        if missing_keys:
            print(f"[MISSING] {len(missing_keys)} keys found in code but missing in {locale_file}:")
            for key in missing_keys:
                print(f"  - {key}")
        else:
            print(f"[OK] All keys found in code exist in {locale_file}.")
        print("-" * 40)

if __name__ == "__main__":
    main()
