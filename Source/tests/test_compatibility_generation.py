"""CI compatibility index tracks the build, never stale GitHub release data."""
from __future__ import annotations

import importlib.util
import tempfile
import unittest
from pathlib import Path

SCRIPT = Path(__file__).resolve().parents[1] / "scripts/generate_compatibility.py"
spec = importlib.util.spec_from_file_location("generate_compatibility", SCRIPT)
generator = importlib.util.module_from_spec(spec)
assert spec and spec.loader
spec.loader.exec_module(generator)


class CompatibilityGenerationTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.repo = Path(self.tmp.name)
        (self.repo / "Source/internal/buildinfo").mkdir(parents=True)
        (self.repo / "Source/internal/buildinfo/buildinfo.go").write_text('package buildinfo\nconst DisplayVersion = "V1.51"\n')
        (self.repo / "Source/protocol.go").write_text('const (\nprotocolVersion = 43\nprotocolMinVersion = 43\nprotocolMaxVersion = 43\n)\n')

    def test_current_index_is_V143_or_later_only(self):
        data = generator.generate(self.repo)
        self.assertEqual([p["id"] for p in data["protocols"]], [43])
        self.assertEqual(data["releases"][0]["modMin"], "V1.43")
        self.assertEqual(data["releases"][0]["bridgeVersion"], "V1.51")
        self.assertEqual(data["releases"][0]["asset"], "Enhanced_PS5_DualSense_Haptics_Bridge_V1.51.zip")

    def test_next_build_auto_updates_tag_and_asset(self):
        (self.repo / "Source/internal/buildinfo/buildinfo.go").write_text('package buildinfo\nconst DisplayVersion = "V1.51"\n')
        data = generator.generate(self.repo)
        self.assertEqual(data["releases"][0]["tag"], "V1.51")
        self.assertEqual(data["releases"][0]["modMax"], "V1.51")
        self.assertEqual(data["releases"][0]["modMin"], "V1.43")

    def test_future_wire_change_requires_compatibility_review(self):
        (self.repo / "Source/protocol.go").write_text('const (\nprotocolVersion = 44\nprotocolMinVersion = 44\nprotocolMaxVersion = 44\n)\n')
        with self.assertRaises(ValueError):
            generator.generate(self.repo)

    def test_rejects_reintroduction_of_legacy_wire_generations(self):
        (self.repo / "Source/protocol.go").write_text('const (\nprotocolVersion = 43\nprotocolMinVersion = 41\nprotocolMaxVersion = 43\n)\n')
        with self.assertRaises(ValueError):
            generator.generate(self.repo)


if __name__ == "__main__":
    unittest.main()
