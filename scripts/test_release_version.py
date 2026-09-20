"""Regression checks for tag-driven metadata stamping (stdlib only)."""
import json
from pathlib import Path
import runpy
import tempfile
import unittest

stamp = runpy.run_path(str(Path(__file__).with_name('set-release-version.py')))['stamp']


class ReleaseVersionTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.files = [self.root / '.claude-plugin/plugin.json',
                      self.root / 'integrations/codex/tokenslim/.codex-plugin/plugin.json']
        for path in self.files:
            path.parent.mkdir(parents=True)
            path.write_text(json.dumps({'name': 'tokenslim', 'version': '0.1.0',
                                        'description': 'Preserve me', 'skills': './skills/'}))
        (self.root / 'VERSION').write_text('0.1.0\n')

    def snapshot(self):
        return [p.read_bytes() for p in [self.root / 'VERSION', *self.files]]

    def test_updates_every_version_and_preserves_metadata(self):
        self.assertEqual(stamp('v2.10.3', self.root), '2.10.3')
        self.assertEqual((self.root / 'VERSION').read_text(), '2.10.3\n')
        for path in self.files:
            self.assertEqual(json.loads(path.read_text()), {
                'name': 'tokenslim', 'version': '2.10.3',
                'description': 'Preserve me', 'skills': './skills/'})
        before = self.snapshot()
        stamp('v2.10.3', self.root)
        self.assertEqual(before, self.snapshot())

    def test_invalid_tags_do_not_change_files(self):
        before = self.snapshot()
        for tag in ['', '1.2.3', 'v01.2.3', 'v1.2', 'v1.2.3-rc.1',
                    'v1.2.3+build', 'v1.2.3\n', 'v1.2.3; echo unsafe']:
            with self.subTest(tag=tag), self.assertRaises(ValueError):
                stamp(tag, self.root)
            self.assertEqual(before, self.snapshot())

    def test_bad_second_manifest_does_not_partially_update(self):
        self.files[1].write_text('{invalid')
        before = self.snapshot()
        with self.assertRaises(ValueError):
            stamp('v2.0.0', self.root)
        self.assertEqual(before, self.snapshot())


if __name__ == '__main__':
    unittest.main()
