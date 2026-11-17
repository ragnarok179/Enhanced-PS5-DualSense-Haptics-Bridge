"""BeamNG Mod Manager IDs must resolve to the packaged mod regardless of ZIP name.

BeamNG core/modmanager.lua:getModNameFromID() searches modData.tagid before
falling back to the internal filename-derived mod name. A missing tagid makes
activation/deactivation fail when the UI uses the metadata ID.
"""
import json
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
MOD_INFO_ROOT = ROOT / "BeamNG-Mod" / "mod_info"


class ModIdentityTests(unittest.TestCase):
    def test_metadata_id_matches_mod_info_path(self):
        info_files = list(MOD_INFO_ROOT.glob("*/info.json"))
        self.assertEqual(len(info_files), 1, "Expected one unambiguous mod_info identity")
        info_file = info_files[0]
        info = json.loads(info_file.read_text(encoding="utf-8"))
        self.assertEqual(
            info.get("tagid"), info_file.parent.name.upper(),
            "The UI uses the mod_info directory ID, while the mod manager only "
            "resolves that ID through modData.tagid (or an exact ZIP basename).",
        )

    def test_mod_id_stays_stable_with_filename_suffix(self):
        info_file = next(MOD_INFO_ROOT.glob("*/info.json"))
        info = json.loads(info_file.read_text(encoding="utf-8"))
        ui_mod_id = info_file.parent.name.upper()
        # BeamNG's getModNameFromPath() normalizes ZIP filenames independently
        # of the folder in mod_info. Appending (7) must not break UI actions.
        for filename in (
            "Enhanced_PS5_DualSense_Haptics_V1.51.zip",
            "Enhanced_PS5_DualSense_Haptics_V1.51 (7).zip",
        ):
            internal_name = filename[:-4].lower()
            self.assertNotEqual(internal_name, ui_mod_id)
            self.assertEqual(info.get("tagid"), ui_mod_id)


if __name__ == "__main__":
    unittest.main()
