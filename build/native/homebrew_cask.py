#!/usr/bin/env python3
"""Generate or update the Homebrew Cask formula for LeviLauncher."""

import argparse
import hashlib
from pathlib import Path
import re


DEFAULT_REPO = "LiteLDev/LeviLauncher"
SHA256_RE = re.compile(r"^[0-9a-fA-F]{64}$")


def read_app_version(config_path: Path, default: str = "1.1.0") -> str:
    if not config_path.is_file():
        return default
    in_info = False
    for line in config_path.read_text(encoding="utf-8").splitlines():
        if re.match(r"^info:\s*$", line):
            in_info = True
            continue
        if in_info and line and not line[0].isspace():
            break
        if in_info:
            match = re.match(r'^\s+version:\s*"([^"]+)"', line)
            if match:
                return match.group(1)
    return default


def normalize_version(raw: str) -> str:
    ver = raw.strip()
    if ver.startswith(("v", "V")):
        ver = ver[1:]
    if not ver or any(ch in ver for ch in ('"', "\n", "\r")):
        raise ValueError(f"invalid version: {raw!r}")
    return ver


def compute_sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as f:
        for chunk in iter(lambda: f.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def render_cask(
    version: str,
    sha256: str | None = None,
    repo: str = DEFAULT_REPO,
) -> str:
    ver = normalize_version(version)
    repo = repo.strip().strip("/")
    if not re.match(r"^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$", repo):
        raise ValueError(f"invalid GitHub repository: {repo!r}")

    if sha256 is None or sha256 == ":no_check":
        sha_line = "  sha256 :no_check"
    else:
        clean = sha256.strip().lower()
        if not SHA256_RE.match(clean):
            raise ValueError(f"invalid sha256 digest: {sha256!r}")
        sha_line = f'  sha256 "{clean}"'

    return (
        'cask "levilauncher" do\n'
        '  arch arm: "arm64"\n'
        "\n"
        f'  version "{ver}"\n'
        f"{sha_line}\n"
        "\n"
        f'  url "https://github.com/{repo}/releases/download/v#{{version}}/LeviLauncher-macos-#{{arch}}-installer.dmg"\n'
        '  name "LeviLauncher"\n'
        '  desc "Minecraft Bedrock Edition launcher"\n'
        f'  homepage "https://github.com/{repo}"\n'
        "\n"
        "  livecheck do\n"
        "    url :url\n"
        "    strategy :github_latest\n"
        "  end\n"
        "\n"
        "  depends_on arch: :arm64\n"
        '  depends_on macos: ">= :sonoma"\n'
        "\n"
        '  app "LeviLauncher.app"\n'
        '  binary "#{appdir}/LeviLauncher.app/Contents/MacOS/LeviLauncher", target: "levilauncher"\n'
        "\n"
        "  zap trash: [\n"
        '    "~/Library/Application Support/LeviLauncher",\n'
        '    "~/Library/Caches/LeviLauncher",\n'
        '    "~/Library/Caches/org.levimc.levilauncher.native",\n'
        '    "~/Library/Logs/LeviLauncher",\n'
        '    "~/Library/Preferences/org.levimc.levilauncher.native.plist",\n'
        '    "~/Library/Saved Application State/org.levimc.levilauncher.native.savedState",\n'
        '    "~/Library/WebKit/org.levimc.levilauncher.native",\n'
        "  ]\n"
        "\n"
        "  caveats <<~EOS\n"
        "    LeviLauncher is currently ad-hoc signed rather than Apple-notarized.\n"
        "    If macOS blocks the app on first launch, install with `--no-quarantine` or run:\n"
        "      xattr -dr com.apple.quarantine /Applications/LeviLauncher.app\n"
        "\n"
        "    The CLI is linked as `levilauncher` (run `levilauncher help` to get started).\n"
        "  EOS\n"
        "end\n"
    )


def main() -> None:
    root = Path(__file__).resolve().parents[2]
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--version",
        default=None,
        help="Release version or tag (defaults to build/config.yml)",
    )
    parser.add_argument(
        "--dmg",
        type=Path,
        default=None,
        help="Path to LeviLauncher-macos-arm64-installer.dmg to hash",
    )
    parser.add_argument(
        "--sha256",
        default=None,
        help="Explicit SHA-256 digest (or :no_check)",
    )
    parser.add_argument(
        "--repo",
        default=DEFAULT_REPO,
        help=f"GitHub owner/repo (default: {DEFAULT_REPO})",
    )
    parser.add_argument(
        "--output",
        type=Path,
        default=root / "bin/levilauncher.rb",
        help="Output cask file path",
    )
    args = parser.parse_args()

    version = args.version or read_app_version(root / "build/config.yml")
    sha256 = args.sha256
    if args.dmg is not None:
        if not args.dmg.is_file():
            parser.error(f"DMG file not found: {args.dmg}")
        sha256 = compute_sha256(args.dmg)

    content = render_cask(version=version, sha256=sha256, repo=args.repo)
    output = args.output.resolve()
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(content, encoding="utf-8")
    print(f"Homebrew Cask: {output}")


if __name__ == "__main__":
    main()
