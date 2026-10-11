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

test('cloud tool directory is optional, isolated, complete and version checked', () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'cc-cloud-tools-'));
  const selected = path.join(dir, 'selected tools');
  const fallback = path.join(dir, 'fallback');
  fs.mkdirSync(selected);
  fs.mkdirSync(fallback);
  const script = path.join(__dirname, 'cloud/env.sh');
  const write = (root, name, output) => {
    fs.writeFileSync(path.join(root, name), `#!/bin/sh\nprintf '%s\\n' '${output}'\n`, { mode: 0o755 });
    fs.chmodSync(path.join(root, name), 0o755);
  };
  const seed = root => {
    write(root, 'node', 'v22.23.3');
    write(root, 'pnpm', '10.34.6');
    write(root, 'go', 'go1.25.13');
    write(root, 'npm', '10.9.8');
  };
  seed(selected);
  seed(fallback);
  const originalPath = process.env.PATH;
  const run = (value, base = fallback) => {
    const env = { ...process.env, PATH: `${base}:${originalPath}`, cloud_root: dir };
    delete env.CLOUD_TOOLCHAIN_BIN;
    if (value !== undefined) env.CLOUD_TOOLCHAIN_BIN = value;
    return spawnSync('sh', ['-c', '. "$1"; command -v node', 'test', script], { env, encoding: 'utf8' });
  };
  try {
    assert.equal(run(undefined).status, 0);
    write(fallback, 'node', 'v24.19.0');
    assert.match(run(undefined).stderr, /Node 22 is required/);
    const chosen = run(selected);
    assert.equal(chosen.status, 0, chosen.stderr);
    assert.equal(chosen.stdout.trim(), path.join(selected, 'node'));
    assert.equal(process.env.PATH, originalPath);
    for (const bad of ['', 'relative', `${selected}:${fallback}`, path.join(dir, 'missing')]) {
      assert.equal(run(bad).status, 1, bad);
    }
    fs.unlinkSync(path.join(selected, 'npm'));
    assert.match(run(selected).stderr, /requires executable npm/);
    seed(selected);
    fs.chmodSync(path.join(selected, 'go'), 0o644);
    assert.match(run(selected).stderr, /requires executable go/);
    seed(selected);
    for (const [tool, bad, message] of [['node', 'v24.0.0', 'Node 22'], ['pnpm', '11.0.0', 'pnpm 10'], ['go', 'go1.26.0', 'Go 1.25.13']]) {
      write(selected, tool, bad);
      assert.equal(run(selected).status, 1);
      assert.ok(run(selected).stderr.includes(message));
      seed(selected);
    }
  } finally {
    fs.rmSync(dir, { recursive: true, force: true });
  }
});
