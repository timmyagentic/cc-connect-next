import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync, existsSync } from 'node:fs';
import { resolve, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { translations, resolveLanguage } from '../public/app.js';
const root = resolve(dirname(fileURLToPath(import.meta.url)), '../public');
const pages = ['index.html', 'docs.html', '404.html'];
test('every local page, image, script and anchor target exists', () => {
  for (const name of pages) {
    const source = readFileSync(resolve(root, name), 'utf8');
    const ids = [...source.matchAll(/\bid="([^"]+)"/g)].map(m => m[1]);
    assert.equal(new Set(ids).size, ids.length, `${name}: unique ids`);
    for (const [, target] of source.matchAll(/(?:src|href)="([^"]+)"/g)) {
      if (target.startsWith('#')) { assert.ok(ids.includes(target.slice(1)), `${name}: ${target}`); continue; }
      if (!target.startsWith('/')) continue;
      const [path, hash] = target.split('#');
      const filename = path === '/' ? 'index.html' : path.includes('.') ? path.slice(1) : path.slice(1)+'.html';
      assert.ok(existsSync(resolve(root, filename)), `${name}: ${target}`);
      if (hash) assert.ok(readFileSync(resolve(root, filename),'utf8').includes(`id="${hash}"`), target);
    }
  }
});
test('all website text bindings have complete Chinese and English translations', () => {
  assert.deepEqual(Object.keys(translations.en).sort(), Object.keys(translations['zh-CN']).sort());
  for (const name of pages) {
    for (const [, key] of readFileSync(resolve(root, name),'utf8').matchAll(/data-i18n="([^"]+)"/g)) {
      assert.ok(translations.en[key]?.trim(), key); assert.ok(translations['zh-CN'][key]?.trim(), key);
    }
  }
  assert.equal(resolveLanguage('zh-TW'), 'zh-CN');
  assert.equal(resolveLanguage('en-US'), 'en');
});
test('the demo is local and migration starts with inspection', () => {
  const home = readFileSync(resolve(root,'index.html'),'utf8');
  const docs = readFileSync(resolve(root,'docs.html'),'utf8');
  assert.match(home, /交互示例 · 不连接你的机器/);
  assert.match(docs, /cc-connect-next migrate --dry-run/);
  assert.doesNotMatch(docs, /<code>[^<]*migrate --switch/);
  assert.doesNotMatch(readFileSync(resolve(root,'app.js'),'utf8'), /fetch\(|WebSocket\(/);
});

test('the staged conversation is rendered with a tab for each local step and a keyboard-accessible next action', () => {
  const page = readFileSync(resolve(root, 'index.html'),'utf8');
  for (const key of ['start','steer','result']) assert.ok(page.includes('data-key="' + key + '"'));
  assert.ok(page.includes('role="tablist"'));
  assert.ok(page.includes('role="tabpanel"'));
  assert.ok(page.includes('<output aria-live="polite">'));
  assert.ok(page.includes('class="button button--primary cc-advance"') || /<button[^>]*class="[^"]*cc-advance[^"]*"[^>]*>/.test(page));
  const source = readFileSync(resolve(root,'../interface/Page.jsx'),'utf8');
  assert.doesNotMatch(source,/fetch\(|WebSocket\(/);
});
