#!/usr/bin/env python3
import hashlib
from pathlib import Path
import tempfile
import unittest

import homebrew_cask


class HomebrewCaskTest(unittest.TestCase):
    def test_render_cask_with_sha256(self):
        digest = "a" * 64
        rendered = homebrew_cask.render_cask(version="v1.2.3", sha256=digest)
        self.assertIn('version "1.2.3"', rendered)
        self.assertIn(f'sha256 "{digest}"', rendered)
        self.assertIn(
            'url "https://github.com/LiteLDev/LeviLauncher/releases/download/v#{version}/LeviLauncher-macos-#{arch}-installer.dmg"',
            rendered,
        )
        self.assertIn(
            'binary "#{appdir}/LeviLauncher.app/Contents/MacOS/LeviLauncher", target: "levilauncher"',
            rendered,
        )

    def test_render_cask_no_check(self):
        rendered = homebrew_cask.render_cask(version="1.1.0", sha256=None)
        self.assertIn("sha256 :no_check", rendered)

    def test_compute_sha256(self):
        with tempfile.TemporaryDirectory() as tmp:
            dmg = Path(tmp) / "test.dmg"
            payload = b"levilauncher-dmg"
            dmg.write_bytes(payload)
            want = hashlib.sha256(payload).hexdigest()
            self.assertEqual(homebrew_cask.compute_sha256(dmg), want)

    def test_invalid_inputs(self):
        with self.assertRaises(ValueError):
            homebrew_cask.render_cask(version="", sha256=None)
        with self.assertRaises(ValueError):
            homebrew_cask.render_cask(version="1.0.0", sha256="not-hex")
        with self.assertRaises(ValueError):
            homebrew_cask.render_cask(version="1.0.0", repo="bad repo")


if __name__ == "__main__":
    unittest.main()
