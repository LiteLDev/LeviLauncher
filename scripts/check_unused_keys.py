import json
import re
from pathlib import Path

ROOT_DIR = Path(__file__).resolve().parents[1]
LOCALES_DIR = ROOT_DIR / "frontend" / "src" / "assets" / "locales"
LOCALE_FILES = sorted(LOCALES_DIR.glob("*.json"))
SEARCH_EXTENSIONS = {".ts", ".tsx", ".go", ".js", ".jsx", ".html"}
ALWAYS_KEEP_PREFIXES = (
    "errors.",
    "file.types.",
    "curseforge.sort.",
    "settings.lip.status.",
    "settings.lip.error.",
)

TRANSLATION_TEMPLATE_CALL_RE = re.compile(
    r"(?:^|[^\w])(?:i18n\.)?t\(\s*`([^`]*\$\{[^`]+?\}[^`]*)`\s*(?:,|\))",
    re.MULTILINE,
)
KEY_TEMPLATE_ASSIGN_RE = re.compile(
    r"\b(?:const|let|var)\s+([A-Za-z_]\w*(?:Key|key))\s*=\s*`([^`]*\$\{[^`]+?\}[^`]*)`",
    re.MULTILINE,
)
PLACEHOLDER_RE = re.compile(r"\$\{[^}]+\}")
KEY_LIKE_TEMPLATE_RE = re.compile(r"^[A-Za-z0-9_.\-${}]+$")


def get_keys(data, prefix=""):
    if not isinstance(data, dict):
        return []

    keys = []
    for key, value in data.items():
        full_key = f"{prefix}.{key}" if prefix else key
        if isinstance(value, dict):
            keys.extend(get_keys(value, full_key))
        else:
            keys.append(full_key)
    return keys


def iter_source_files():
    search_roots = [
        ROOT_DIR / "frontend" / "src",
        ROOT_DIR / "internal",
    ]

    for path in sorted(ROOT_DIR.iterdir()):
        if path.is_file() and path.suffix in SEARCH_EXTENSIONS:
            yield path

    frontend_dir = ROOT_DIR / "frontend"
    if frontend_dir.exists():
        for path in sorted(frontend_dir.iterdir()):
            if path.is_file() and path.suffix in SEARCH_EXTENSIONS:
                yield path

    for root in search_roots:
        if not root.exists():
            continue
        for path in sorted(root.rglob("*")):
            if path.is_file() and path.suffix in SEARCH_EXTENSIONS:
                yield path


def load_all_keys():
    all_keys = set()
    for locale_file in LOCALE_FILES:
        with locale_file.open(encoding="utf-8") as file:
            all_keys.update(get_keys(json.load(file)))
    return all_keys


def looks_like_translation_key_template(template):
    return (
        "${" in template
        and "." in template
        and KEY_LIKE_TEMPLATE_RE.fullmatch(template) is not None
    )


def build_key_pattern(template):
    parts = PLACEHOLDER_RE.split(template)
    placeholders = PLACEHOLDER_RE.findall(template)
    regex_parts = []

    for index, part in enumerate(parts):
        regex_parts.append(re.escape(part))
        if index < len(placeholders):
            regex_parts.append(r"[A-Za-z0-9_.-]+")

    return re.compile(rf"^{''.join(regex_parts)}$")


def collect_dynamic_templates(content):
    templates = set()

    for match in TRANSLATION_TEMPLATE_CALL_RE.finditer(content):
        template = match.group(1).strip()
        if looks_like_translation_key_template(template):
            templates.add(template)

    template_vars = {}
    for match in KEY_TEMPLATE_ASSIGN_RE.finditer(content):
        var_name = match.group(1)
        template = match.group(2).strip()
        if looks_like_translation_key_template(template):
            template_vars[var_name] = template

    for var_name, template in template_vars.items():
        usage_re = re.compile(
            rf"(?:^|[^\w])(?:i18n\.)?t\(\s*{re.escape(var_name)}\s*(?:,|\))",
            re.MULTILINE,
        )
        if usage_re.search(content):
            templates.add(template)

    return templates


def collect_used_keys(all_keys):
    used_keys = set()
    dynamic_templates = set()

    for key in all_keys:
        if any(key.startswith(prefix) for prefix in ALWAYS_KEEP_PREFIXES):
            used_keys.add(key)

    for source_file in iter_source_files():
        content = source_file.read_text(encoding="utf-8", errors="ignore")

        for key in all_keys - used_keys:
            if (
                f'"{key}"' in content
                or f"'{key}'" in content
                or f"`{key}`" in content
            ):
                used_keys.add(key)

        dynamic_templates.update(collect_dynamic_templates(content))

    for template in dynamic_templates:
        key_pattern = build_key_pattern(template)
        for key in all_keys - used_keys:
            if key_pattern.fullmatch(key):
                used_keys.add(key)

    return used_keys, dynamic_templates


def main():
    all_keys = load_all_keys()
    used_keys, dynamic_templates = collect_used_keys(all_keys)
    unused_keys = all_keys - used_keys

    print(f"Locale files scanned: {len(LOCALE_FILES)}")
    print(f"Dynamic templates detected: {len(dynamic_templates)}")
    print(f"Unused keys found: {len(unused_keys)}")

    for key in sorted(unused_keys):
        print(key)


if __name__ == "__main__":
    main()
