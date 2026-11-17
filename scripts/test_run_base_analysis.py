"""Exercise base-commit analysis against real Git repositories and a fake analyzer."""
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

RUNNER = Path(__file__).with_name('run-base-analysis.sh').resolve()

ANALYZER = '''#!/bin/bash
printf '%s\\n' "$PWD" > "$ANALYZER_LOG"
printf '%s\\n' "$@" >> "$ANALYZER_LOG"
for config in .columbo.yml custom/columbo.yml; do
  if [[ -f $config ]]; then printf 'config %s: %s\\n' "$config" "$(cat "$config")" >> "$ANALYZER_LOG"; fi
done
printf 'analyzed-sha %s\\n' "$(git rev-parse HEAD)" >> "$ANALYZER_LOG"
test -f base-only.go || exit 2
printf snapshot > "$4"
exit "$ANALYZER_EXIT"
'''


def git(*args, cwd):
    subprocess.run(['git', *args], cwd=cwd, check=True, capture_output=True, text=True)


def rev_parse(cwd):
    return subprocess.run(['git', 'rev-parse', 'HEAD'], cwd=cwd, check=True,
                          capture_output=True, text=True).stdout.strip()


class BaseAnalysisTests(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory(prefix='columbo base ')
        root = Path(self.directory.name)
        self.root = root
        author = root / 'author'
        (author / 'module').mkdir(parents=True)
        git('init', '-q', '-b', 'main', cwd=author)
        git('config', 'user.email', 'test@example.invalid', cwd=author)
        git('config', 'user.name', 'Test', cwd=author)
        (author / 'module' / 'base-only.go').write_text('package module\n')
        (author / 'module' / '.columbo.yml').write_text('base config\n')
        git('add', '.', cwd=author)
        git('commit', '-qm', 'base', cwd=author)
        self.base_sha = rev_parse(author)
        (author / 'module' / '.columbo.yml').write_text('head config\n')
        (author / 'module' / 'custom').mkdir()
        (author / 'module' / 'custom' / 'columbo.yml').write_text('custom head config\n')
        git('add', '.', cwd=author)
        git('commit', '-qm', 'head', cwd=author)
        git('clone', '-q', '--bare', str(author), str(root / 'origin.git'), cwd=root)
        git('clone', '-q', '--depth=1', 'file://' + str(root / 'origin.git'), str(root / 'workspace'), cwd=root)
        self.evidence = root / 'evidence'
        self.evidence.mkdir()
        self.temp = root / 'temp'
        self.temp.mkdir()
        self.analyzer = root / 'bin' / 'columbo'
        self.analyzer.parent.mkdir()
        self.analyzer.write_text(ANALYZER)
        self.analyzer.chmod(0o755)

    def checkout_merge(self):
        author = self.root / 'author'
        pr_head_sha = rev_parse(author)
        git('branch', 'pr-head', cwd=author)
        git('reset', '--hard', self.base_sha, cwd=author)
        (author / 'module' / 'main-only.go').write_text('package module\n')
        git('add', '.', cwd=author)
        git('commit', '-qm', 'main advanced', cwd=author)
        current_base_sha = rev_parse(author)
        git('merge', '-q', '--no-ff', '-m', 'merge PR', 'pr-head', cwd=author)
        git('push', '-q', str(self.root / 'origin.git'), 'main', cwd=author)
        workspace = self.root / 'workspace'
        git('fetch', '-q', '--depth=1', 'origin', 'main', cwd=workspace)
        git('checkout', '-q', '--detach', 'FETCH_HEAD', cwd=workspace)
        return pr_head_sha, current_base_sha

    def tearDown(self):
        self.directory.cleanup()

    def run_base(self, analyzer_exit=1, config='', base_sha=None, working_directory='module', pr_head_sha=''):
        outputs = self.root / 'outputs'
        outputs.write_text('')
        env = dict(os.environ, RUNNER_TEMP=str(self.temp), GITHUB_OUTPUT=str(outputs),
                   GITHUB_WORKSPACE=str(self.root / 'workspace'),
                   COLUMBO_WORKING_DIRECTORY=working_directory, COLUMBO_CONFIG=config,
                   COLUMBO_BASE_SHA=self.base_sha if base_sha is None else base_sha,
                   COLUMBO_PR_HEAD_SHA=pr_head_sha,
                   COLUMBO_ANALYZER=str(self.analyzer), COLUMBO_EVIDENCE=str(self.evidence),
                   ANALYZER_LOG=str(self.root / 'analyzer.log'), ANALYZER_EXIT=str(analyzer_exit))
        result = subprocess.run(['bash', str(RUNNER)], env=env, capture_output=True, text=True)
        self.assertEqual(0, result.returncode, result.stderr)
        return dict(line.split('=', 1) for line in outputs.read_text().splitlines()), result.stdout

    def analyzer_log(self):
        return (self.root / 'analyzer.log').read_text().splitlines()

    def test_findings_at_base_yield_a_snapshot_analyzed_with_the_head_config(self):
        outputs, _ = self.run_base(analyzer_exit=1)
        self.assertEqual({'base-snapshot': str(self.evidence / 'base.sqlite')}, outputs)
        log = self.analyzer_log()
        self.assertTrue(log[0].endswith('/src/module'), log[0])
        self.assertEqual(['--staged=false', '--no-history', '--output', str(self.evidence / 'base.sqlite'), './...'], log[1:6])
        self.assertIn('config .columbo.yml: head config', log)
        self.assertEqual('1\n', (self.evidence / 'base-exit-status.txt').read_text())

    def test_clean_base_yields_a_snapshot(self):
        outputs, _ = self.run_base(analyzer_exit=0)
        self.assertEqual(str(self.evidence / 'base.sqlite'), outputs['base-snapshot'])

    def test_configured_path_is_copied_from_head_and_passed_through(self):
        self.run_base(config='custom/columbo.yml')
        log = self.analyzer_log()
        self.assertEqual(['--config', 'custom/columbo.yml', './...'], log[5:8])
        self.assertIn('config custom/columbo.yml: custom head config', log)

    def test_head_without_config_analyzes_base_with_defaults(self):
        (self.root / 'workspace' / 'module' / '.columbo.yml').unlink()
        self.run_base()
        self.assertFalse(any(line.startswith('config .columbo.yml') for line in self.analyzer_log()))

    def test_analysis_error_marks_base_unavailable(self):
        outputs, stdout = self.run_base(analyzer_exit=2)
        self.assertEqual({'base-status': 'unavailable'}, outputs)
        self.assertIn('::warning::Columbo feedback skipped', stdout)

    def test_unreachable_base_commit_marks_base_unavailable(self):
        outputs, stdout = self.run_base(base_sha='0' * 40)
        self.assertEqual({'base-status': 'unavailable'}, outputs)
        self.assertIn('fetching the base commit failed', stdout)

    def test_missing_configured_file_marks_base_unavailable(self):
        outputs, _ = self.run_base(config='absent.yml')
        self.assertEqual({'base-status': 'unavailable'}, outputs)

    def test_merge_checkout_uses_current_base_parent(self):
        pr_head_sha, current_base_sha = self.checkout_merge()
        workspace = self.root / 'workspace'
        shallow_parent = subprocess.run(['git', 'rev-parse', '--verify', 'HEAD^1'],
                                        cwd=workspace, capture_output=True, text=True)
        self.assertNotEqual(0, shallow_parent.returncode)
        outputs, _ = self.run_base(pr_head_sha=pr_head_sha)
        self.assertEqual(str(self.evidence / 'base.sqlite'), outputs['base-snapshot'])
        self.assertIn('analyzed-sha ' + current_base_sha, self.analyzer_log())

    def test_unrelated_merge_checkout_keeps_event_base(self):
        _, _ = self.checkout_merge()
        outputs, _ = self.run_base(pr_head_sha='0' * 40)
        self.assertEqual(str(self.evidence / 'base.sqlite'), outputs['base-snapshot'])
        self.assertIn('analyzed-sha ' + self.base_sha, self.analyzer_log())

    def test_pr_head_checkout_keeps_event_base(self):
        head_sha = rev_parse(self.root / 'workspace')
        outputs, _ = self.run_base(pr_head_sha=head_sha)
        self.assertEqual(str(self.evidence / 'base.sqlite'), outputs['base-snapshot'])
        self.assertIn('analyzed-sha ' + self.base_sha, self.analyzer_log())

    def test_worktree_is_removed_after_analysis(self):
        self.run_base()
        listing = subprocess.run(['git', 'worktree', 'list'], cwd=self.root / 'workspace',
                                 capture_output=True, text=True, check=True).stdout
        self.assertEqual(1, len(listing.splitlines()), listing)


if __name__ == '__main__':
    unittest.main()
