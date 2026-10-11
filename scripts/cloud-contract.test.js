const test = require('node:test');
const assert = require('node:assert/strict');
const { spawnSync } = require('node:child_process');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');

test('cloud check rejects invalid SHA, wrong HEAD and dirty checkout before setup', () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'cc-cloud-contract-'));
  try {
    fs.mkdirSync(path.join(dir, 'scripts/cloud'), { recursive: true });
    fs.copyFileSync(path.join(__dirname, 'cloud/check.sh'), path.join(dir, 'scripts/cloud/check.sh'));
    const git = (...args) => {
      const r = spawnSync('git', args, { cwd: dir, encoding: 'utf8' });
      assert.equal(r.status, 0, r.stderr);
      return r.stdout.trim();
    };
    git('init');
    git('add', '.');
    git('-c', 'user.name=Cloud Test', '-c', 'user.email=cloud-test@example.invalid', 'commit', '-m', 'fixture');
    const run = sha => spawnSync('sh', ['scripts/cloud/check.sh', sha], { cwd: dir, encoding: 'utf8' });
    assert.equal(run('main').status, 2);
    const wrong = run('0'.repeat(40));
    assert.equal(wrong.status, 1);
    assert.match(wrong.stderr, /SHA mismatch/);
    fs.writeFileSync(path.join(dir, 'untracked.txt'), 'dirty');
    const dirty = run(git('rev-parse', 'HEAD'));
    assert.equal(dirty.status, 1);
    assert.match(dirty.stderr, /Clean checkout required/);
  } finally {
    fs.rmSync(dir, { recursive: true, force: true });
  }
});
