import importlib.util
import io
import json
import sys
import shutil
import tarfile
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
from release_lib import (TARGETS, archive_name, digest, make_archive, render_formula,
                         require_clean_worktree, validate_repository, validate_version,
                         write_checksums, check_embedded_inputs)


class ReleaseTests(unittest.TestCase):
    def test_stable_versions(self):
        for value in ("0.7.0", "1.0.0", "12.3.456"):
            self.assertEqual(validate_version(value), value)
        for value in ("v0.7.0", "01.0.0", "1.2", "1.2.3-rc.1", "1.2.3\n", "$(whoami)"):
            with self.assertRaises(ValueError): validate_version(value)

    def test_repository_is_not_a_url_or_shell_code(self):
        self.assertEqual(validate_repository("example-user/nearprod"), "example-user/nearprod")
        for value in ("", "https://github.com/a/b", "a/b.git", "a/b/extra", "a/b;rm", 'a/"evil', "a/b\n"):
            with self.assertRaises(ValueError): validate_repository(value)

    def test_public_release_requires_clean_worktree(self):
        require_clean_worktree("")
        require_clean_worktree("\n")
        for status in (" M README.md\n", "?? unexpected.txt\n"):
            with self.assertRaises(ValueError):
                require_clean_worktree(status)

    def test_archive_is_reproducible_and_executable(self):
        with tempfile.TemporaryDirectory() as tmp:
            a, b = Path(tmp) / "a.tgz", Path(tmp) / "b.tgz"
            items = {"nearprod": (b"test executable", 0o755), "LICENSE": (b"unchanged terms", 0o644)}
            make_archive(a, items, 123)
            make_archive(b, items, 123)
            self.assertEqual(digest(a), digest(b))
            with tarfile.open(a) as archive:
                self.assertEqual(archive.getmember("nearprod").mode, 0o755)
                self.assertEqual(archive.getmember("LICENSE").uid, 0)
                self.assertEqual(archive.getnames(), ["LICENSE", "nearprod"])

    def test_archive_rejects_traversal(self):
        with tempfile.TemporaryDirectory() as tmp:
            with self.assertRaises(ValueError):
                make_archive(Path(tmp) / "unsafe.tgz", {"../bad": (b"x", 0o644)}, 0)

    def test_formula_has_real_hashes_for_four_targets(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "build-info.json").write_text(json.dumps({"version": "0.7.0", "snapshot": False, "prebuiltUI": False}))
            for system, arch in TARGETS:
                (root / archive_name("0.7.0", system, arch)).write_bytes(f"fixture {system}/{arch}".encode())
            formula = render_formula(root, "example-user/nearprod")
            self.assertEqual(formula.count("sha256 "), 4)
            for file in root.glob("*.tar.gz"):
                self.assertIn(digest(file), formula)
            self.assertIn("bin.install", formula)
            self.assertIn('license "MIT"', formula)
            self.assertNotIn('depends_on "node"', formula)
            self.assertNotIn("def post_install", formula)
            write_checksums(root)
            self.assertEqual(len((root / "SHA256SUMS.txt").read_text().splitlines()), 5)

    def test_snapshot_cannot_generate_a_formula(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "build-info.json").write_text(json.dumps({"version": "0.7.0-dev", "snapshot": True}))
            with self.assertRaises(ValueError): render_formula(root, "example-user/nearprod")

    def test_windows_is_not_promised(self):
        with self.assertRaises(ValueError): archive_name("0.7.0", "windows", "amd64")

    def test_embedded_secrets_and_extra_assets_are_rejected(self):
        source = Path(__file__).resolve().parents[2]
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            for relative in ("internal/webui/dist", "ui/src", "ui/public"):
                shutil.copytree(source / relative, root / relative)
            check_embedded_inputs(root)
            dist = root / "internal/webui/dist"
            for name in (".env.release-test", "unexpected-release-test.txt"):
                marker = dist / name
                marker.write_text("synthetic test only")
                try:
                    with self.assertRaises(ValueError): check_embedded_inputs(root)
                finally:
                    marker.unlink()

    def test_missing_archive_fails_closed(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "build-info.json").write_text(json.dumps({"version": "0.7.0", "snapshot": False}))
            with self.assertRaises(ValueError): render_formula(root, "example-user/nearprod")


if __name__ == "__main__":
    unittest.main()
