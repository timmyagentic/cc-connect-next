#!/usr/bin/env python3
"""Build and serve a private loopback npm/standalone update fixture.

The binaries use the production Engine, platform and update implementation.
Only release metadata/download transport and the model process are fixtures.
The server delays archive transfer so the actual npm subprocess remains busy.
It never reads credentials, publishes packages or changes global npm config.
"""
import argparse
import base64
import hashlib
import http.server
import json
import os
from pathlib import Path
import shutil
import subprocess
import tarfile
import time

parser = argparse.ArgumentParser()
parser.add_argument('--root', type=Path, required=True)
parser.add_argument('--delay', type=float, default=30)
args = parser.parse_args()
root = args.root.resolve()
if not root.name.startswith('cc-update-qa-') or root.exists():
    parser.error('root must be a new cc-update-qa-* directory')
root.mkdir(mode=0o700)
repo = Path(__file__).resolve().parents[3]
versions = ('v0.0.0-qa.1', 'v0.0.0-qa.2')
prefix = root / 'prefix'
package_dir = prefix / 'lib/node_modules/cc-connect-next'
(package_dir / 'bin').mkdir(parents=True)
(root / 'artifacts').mkdir()
server_dir = root / 'server'
server_dir.mkdir()

class Handler(http.server.BaseHTTPRequestHandler):
    def log_message(self, *_args):
        pass

    def do_GET(self):
        route = self.path.split('?', 1)[0]
        if route == '/cc-connect-next':
            data = metadata
        elif route == '/release.json':
            data = release_json
        else:
            name = route.rsplit('/', 1)[-1]
            path = server_dir / name
            if not path.is_file():
                self.send_error(404)
                return
            data = path.read_bytes()
        self.send_response(200)
        self.send_header('Content-Length', str(len(data)))
        self.send_header('Content-Type', 'application/json' if route.endswith('json') or route == '/cc-connect-next' else 'application/octet-stream')
        self.end_headers()
        try:
            if route.endswith('.tar.gz') and args.delay > 0:
                chunk_size = max(1, len(data) // 120)
                for start in range(0, len(data), chunk_size):
                    self.wfile.write(data[start:start + chunk_size])
                    self.wfile.flush()
                    time.sleep(args.delay / 120)
            else:
                self.wfile.write(data)
        except (BrokenPipeError, ConnectionResetError):
            pass

server = http.server.ThreadingHTTPServer(('127.0.0.1', 0), Handler)
origin = f'http://127.0.0.1:{server.server_port}'
for index, version in enumerate(versions):
    output = package_dir / 'bin/cc-connect-next' if index == 0 else root / 'target'
    subprocess.run(['go', 'build', '-tags=updateqa', '-ldflags', f'-s -w -X main.version={version}', '-o', str(output), './tests/manual/updateqa'], cwd=repo, check=True)

platform = subprocess.check_output(['go', 'env', 'GOOS'], text=True).strip()
arch = subprocess.check_output(['go', 'env', 'GOARCH'], text=True).strip()
archive_name = f'cc-connect-next-{versions[1]}-{platform}-{arch}.tar.gz'
binary_name = archive_name.removesuffix('.tar.gz')
archive = server_dir / archive_name
with tarfile.open(archive, 'w:gz') as tf:
    tf.add(root / 'target', arcname=binary_name)
(server_dir / 'checksums.txt').write_text(hashlib.sha256(archive.read_bytes()).hexdigest() + '  ' + archive_name + '\n')

base_package = json.loads((repo / 'npm/package.json').read_text())
base_package['version'] = versions[0].removeprefix('v')
(package_dir / 'package.json').write_text(json.dumps(base_package))
shutil.copy(repo / 'npm/run.js', package_dir / 'run.js')
(prefix / 'bin').mkdir()
(prefix / 'bin/cc-connect-next').symlink_to('../lib/node_modules/cc-connect-next/run.js')

stage = root / 'package'
stage.mkdir()
base_package['version'] = versions[1].removeprefix('v')
(stage / 'package.json').write_text(json.dumps(base_package))
shutil.copy(repo / 'npm/run.js', stage / 'run.js')
install = (repo / 'npm/install.js').read_text()
old = 'https://github.com/${GITHUB_REPO}/releases/download/${VERSION}'
assert install.count(old) == 2
(stage / 'install.js').write_text(install.replace(old, origin + '/assets'))
tarball = server_dir / 'cc-connect-next-0.0.0-qa.2.tgz'
with tarfile.open(tarball, 'w:gz') as tf:
    tf.add(stage, arcname='package')
package_bytes = tarball.read_bytes()
base_package['dist'] = {'tarball': origin + '/' + tarball.name, 'shasum': hashlib.sha1(package_bytes).hexdigest(), 'integrity': 'sha512-' + base64.b64encode(hashlib.sha512(package_bytes).digest()).decode()}
metadata = json.dumps({'name': base_package['name'], 'dist-tags': {'latest': base_package['version']}, 'versions': {base_package['version']: base_package}}).encode()
release_json = json.dumps({'Tag': versions[1], 'URL': 'https://github.com/timmyagentic/cc-connect-next', 'Prerelease': True,
 'Notes': '## 中文\n### 更新亮点\n- 一张卡看清更新进度和已用时间\n- 等待已有任务结束，避免中断工作\n- 重启后核对版本，原卡恢复完成状态\n### 升级注意\n隔离 QA 版本，仅用于真实飞书更新验收。\n## English\n### Highlights\n- One update card with live stage and elapsed time\n- Existing tasks finish before the update\n- Verify the running version after restart\n### Upgrade notes\nIsolated QA fixture, not a published release.',
 'Assets': [{'Name': name, 'DownloadURL': origin + '/assets/' + name, 'Size': (server_dir/name).stat().st_size} for name in (archive_name, 'checksums.txt')]}).encode()
(root / 'fixture.json').write_text(json.dumps({'origin':origin,'executable':str(package_dir/'bin/cc-connect-next'),'target_sha256':hashlib.sha256((root/'target').read_bytes()).hexdigest(),'archive_sha256':hashlib.sha256(archive.read_bytes()).hexdigest(),'version_from':versions[0],'version_to':versions[1]}, indent=2))
print(json.dumps({'ready':True,'root':str(root),'origin':origin}), flush=True)
try:
    server.serve_forever()
except KeyboardInterrupt:
    server.server_close()
