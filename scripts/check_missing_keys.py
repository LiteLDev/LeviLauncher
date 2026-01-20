#!/usr/bin/env python3
"""
Check for missing keys in other locale files compared to en_US.

Usage:
  python scripts/check_missing_keys.py
"""

import json
import os
import sys
from typing import Dict, Set

# Determine paths
SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
PROJECT_ROOT = os.path.dirname(SCRIPT_DIR)
LOCALES_DIR = os.path.join(PROJECT_ROOT, "frontend", "src", "assets", "locales")
EN_US_PATH = os.path.join(LOCALES_DIR, "en_US.json")

def load_json(path: str) -> Dict:
    if not os.path.exists(path):
        print(f"Error: File not found: {path}")
        sys.exit(1)
    try:
        with open(path, "r", encoding="utf-8") as f:
            return json.load(f)
    except json.JSONDecodeError as e:
        print(f"Error: Failed to parse JSON {path}: {e}")
        sys.exit(1)

def flatten_map(data: object, prefix: str = "") -> Dict[str, object]:
    """Flatten nested dict/list into a mapping of dot-separated paths -> leaf values."""
    acc: Dict[str, object] = {}
    if isinstance(data, dict):
        for k, v in data.items():
            pfx = f"{prefix}.{k}" if prefix else k
            if isinstance(v, (dict, list)):
                acc.update(flatten_map(v, pfx))
            else:
                acc[pfx] = v
    elif isinstance(data, list):
        for i, v in enumerate(data):
            pfx = f"{prefix}.{i}" if prefix else str(i)
            if isinstance(v, (dict, list)):
                acc.update(flatten_map(v, pfx))
            else:
                acc[pfx] = v
    return acc

def main():
    if not os.path.exists(LOCALES_DIR):
        print(f"Error: Locales directory not found at {LOCALES_DIR}")
        sys.exit(1)

    print(f"Loading base locale: {EN_US_PATH}")
    en_us_data = load_json(EN_US_PATH)
    en_us_flat = flatten_map(en_us_data)
    en_us_keys: Set[str] = set(en_us_flat.keys())

    print(f"Base locale (en_US) has {len(en_us_keys)} keys.\n")

    # Iterate over other locale files
    found_issues = False
    for filename in sorted(os.listdir(LOCALES_DIR)):
        if not filename.endswith(".json") or filename == "en_US.json":
            continue
        
        file_path = os.path.join(LOCALES_DIR, filename)
        print(f"Checking {filename}...")
        
        target_data = load_json(file_path)
        target_flat = flatten_map(target_data)
        target_keys: Set[str] = set(target_flat.keys())
        
        missing_keys = sorted(list(en_us_keys - target_keys))
        
        if missing_keys:
            found_issues = True
            print(f"  [MISSING] {len(missing_keys)} keys missing in {filename}:")
            for key in missing_keys:
                print(f"    - {key}")
        else:
            print(f"  [OK] No missing keys.")
        print("")

    if found_issues:
        sys.exit(1)
    else:
        print("All locales are up to date with en_US.")
        sys.exit(0)

if __name__ == "__main__":
    main()
