"""Exercise the action's shell runner with controlled build and analysis outcomes."""
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

RUNNER = Path(__file__).with_name('run-action.sh').resolve()


class ActionRunnerTests(unittest.TestCase):
    def run_case(self, analysis_exit, build_exit=0):
        with tempfile.TemporaryDirectory(prefix='columbo action ') as directory:
            root = Path(directory)
            (root / 'module').mkdir()
            analyzer = root / 'analyzer'
            analyzer.write_text('#!/bin/bash\n'
                                'test "$PWD" = "$GITHUB_WORKSPACE/module" || exit 2\n'
                                'test "$4" = "settings with spaces.yml" || exit 2\n'
                                'printf evidence > "$2"\n'
                                'echo "analysis completed"\n'
                                f'exit {analysis_exit}\n')
            analyzer.chmod(0o755)
            go = root / 'go'
            go.write_text('#!/bin/bash\n'
                          'if [[ "$1" == version ]]; then echo "go version test"; exit 0; fi\n'
                          f'if [[ {build_exit} != 0 ]]; then echo "build failed"; exit {build_exit}; fi\n'
                          'cp "$FAKE_ANALYZER" "$5"\n')
            go.chmod(0o755)
            output = root / 'outputs'
            env = dict(os.environ, PATH=f'{root}:{os.environ["PATH"]}',
                       RUNNER_TEMP=str(root), GITHUB_OUTPUT=str(output),
                       GITHUB_SHA='abc', GITHUB_ACTION_PATH=str(root),
                       GITHUB_WORKSPACE=str(root), COLUMBO_WORKING_DIRECTORY='module',
                       COLUMBO_CONFIG='settings with spaces.yml', FAKE_ANALYZER=str(analyzer))
            result = subprocess.run(['bash', str(RUNNER)], env=env, capture_output=True, text=True)
            self.assertEqual(result.returncode, 0, result.stderr)
            outputs = dict(line.split('=', 1) for line in output.read_text().splitlines())
            evidence = Path(outputs['evidence'])
            expected = 2 if build_exit else analysis_exit
            self.assertEqual(outputs['exit-code'], str(expected))
            self.assertEqual((evidence / 'exit-status.txt').read_text(), f'{expected}\n')
            self.assertTrue((evidence / 'provenance.txt').exists())
            self.assertTrue((evidence / 'build.txt').exists())
            self.assertEqual((evidence / 'report.sqlite').exists(), build_exit == 0)
            if not build_exit:
                self.assertIn('analysis completed', (evidence / 'report.txt').read_text())

    def test_clean_analysis(self):
        self.run_case(0)

    def test_findings_still_leave_evidence_for_publishing(self):
        self.run_case(1)

    def test_analysis_error_is_preserved(self):
        self.run_case(2)

    def test_build_failure_records_error_without_snapshot(self):
        self.run_case(0, build_exit=1)


if __name__ == '__main__':
    unittest.main()
