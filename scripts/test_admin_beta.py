import importlib.util
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

SCRIPT = Path(__file__).resolve().parents[1] / 'admin-beta.py'
if not SCRIPT.exists():
    SCRIPT = Path(__file__).resolve().parent / 'admin-beta.py'
spec = importlib.util.spec_from_file_location('admin_beta', SCRIPT)
admin = importlib.util.module_from_spec(spec)
spec.loader.exec_module(admin)


class AdminBetaTests(unittest.TestCase):
    def test_beta_increments_same_line_and_rolls_after_stable(self):
        with patch.object(admin, 'REPO', 'timmyagentic/cc-connect-next'):
            self.assertEqual(admin.next_version('0.3.7-beta.3', ['v0.3.6', 'v0.3.7-beta.3']), ('0.3.7', 4))
            self.assertEqual(admin.next_version('0.3.7', ['v0.3.7']), ('0.3.8', 1))
            self.assertEqual(admin.next_version('0.3.6-beta.2', ['v0.3.7-beta.5']), ('0.3.7', 6))

    def test_private_beta_uses_private_feed_and_never_regresses(self):
        with patch.object(admin, 'REPO', 'timmyagentic/quota-monitor'):
            self.assertEqual(admin.next_version('1.0.16', ['v1.0.15'], '1.0.16-beta.4'), ('1.0.16', 5))
            self.assertEqual(admin.next_version('1.0.15', ['v1.0.15'], '1.0.17-beta.4'), ('1.0.17', 5))

    def test_changed_head_stops_before_merge(self):
        with patch.object(admin, 'gh', return_value={'headRefOid': 'new', 'isDraft': False, 'baseRefName': 'main'}):
            with self.assertRaisesRegex(RuntimeError, 'changed'):
                admin.check_pr(5, 'old')

    def test_closed_pr_is_not_silently_skipped(self):
        with patch.object(admin, 'gh', return_value={'headRefOid': 'old', 'isDraft': False, 'baseRefName': 'main', 'state': 'CLOSED'}):
            with self.assertRaisesRegex(RuntimeError, 'no longer open'):
                admin.check_pr(5, 'old')

    def test_invalid_and_exhausted_versions_fail(self):
        with self.assertRaises(ValueError):
            admin.next_version('invalid', [])
        with self.assertRaises(ValueError):
            admin.next_version('1.0.0', [], '1.0.0-beta.8999')


if __name__ == '__main__':
    unittest.main()
