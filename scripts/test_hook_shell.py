"""Ensure Windows hook checks cannot accidentally use the WSL Bash launcher."""
import os
from pathlib import Path
import runpy
import tempfile
import unittest
from unittest.mock import patch

select_bash = runpy.run_path(str(Path(__file__).with_name('check-hook-command.py')))['select_bash']


class HookShellTests(unittest.TestCase):
    def test_explicit_runner_shell(self):
        with tempfile.TemporaryDirectory() as temp:
            bash = Path(temp) / 'bash.exe'
            bash.touch()
            with patch.dict(os.environ, {'TOKENSLIM_TEST_BASH': str(bash)}, clear=True):
                self.assertEqual(select_bash(windows=True), bash)

    def test_git_bash_wins_over_path_bash(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp) / 'Git with spaces'
            bash = root / 'bin/bash.exe'
            bash.parent.mkdir(parents=True)
            bash.touch()
            git = root / 'cmd/git.exe'
            git.parent.mkdir()
            git.touch()
            def which(name):
                return str(git) if name == 'git' else 'C:/Windows/System32/bash.exe'
            with patch.dict(os.environ, {}, clear=True), patch('shutil.which', side_effect=which):
                self.assertEqual(select_bash(windows=True).resolve(), bash.resolve())

    def test_windows_does_not_fall_back_to_wsl(self):
        def which(name):
            return 'C:/Windows/System32/bash.exe' if name == 'bash' else None
        with patch.dict(os.environ, {}, clear=True), patch('shutil.which', side_effect=which):
            with self.assertRaisesRegex(RuntimeError, 'Git Bash not found'):
                select_bash(windows=True)

    def test_invalid_override_is_reported(self):
        with patch.dict(os.environ, {'TOKENSLIM_TEST_BASH': 'relative/bash.exe'}, clear=True):
            with self.assertRaisesRegex(RuntimeError, 'absolute Bash path'):
                select_bash(windows=True)


if __name__ == '__main__':
    unittest.main()
