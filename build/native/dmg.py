#!/usr/bin/env python3
"""Package a signed LeviLauncher app as a drag-to-install macOS disk image."""

import argparse
from pathlib import Path
import subprocess
import sys
import tempfile


def main():
    bin_dir = Path(__file__).resolve().parents[2] / "bin"
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--app", type=Path, default=bin_dir / "LeviLauncher.app")
    parser.add_argument(
        "--output", type=Path, default=bin_dir / "LeviLauncher-macos-installer.dmg"
    )
    args = parser.parse_args()
    if sys.platform != "darwin":
        parser.error("DMG packaging requires macOS.")

    app = args.app.resolve()
    output = args.output.resolve()
    if app.suffix != ".app" or not (app / "Contents/Info.plist").is_file():
        parser.error(f"macOS application bundle not found: {app}")
    if output.suffix != ".dmg" or app in output.parents:
        parser.error("--output must be a .dmg path outside the application bundle.")

    subprocess.run(["codesign", "--verify", "--deep", "--strict", str(app)], check=True)
    output.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix=".levilauncher-dmg-", dir=output.parent) as temp:
        work = Path(temp)
        contents = work / "contents"
        contents.mkdir()
        subprocess.run(["ditto", str(app), str(contents / app.name)], check=True)
        (contents / "Applications").symlink_to("/Applications")

        image = work / output.name
        subprocess.run(
            [
                "hdiutil", "create", "-srcfolder", str(contents), "-volname", "LeviLauncher",
                "-fs", "HFS+", "-format", "UDZO", str(image),
            ],
            check=True,
        )
        subprocess.run(["hdiutil", "verify", str(image)], check=True)
        image.replace(output)
    print(f"macOS installer: {output}")


if __name__ == "__main__":
    main()
