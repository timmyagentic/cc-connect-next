"use strict";

const assert = require("node:assert/strict");
const crypto = require("node:crypto");
const { execFile } = require("node:child_process");
const http = require("node:http");
const { promisify } = require("node:util");
const test = require("node:test");

const {
  parseChecksumManifest,
  verifyChecksum,
} = require("./install.js");

const filename = "cc-connect-next-v0.1.0-beta.1-darwin-arm64.tar.gz";
const payload = Buffer.from("verified archive bytes");
const digest = crypto.createHash("sha256").update(payload).digest("hex");

test("fetch drains redirect bodies so the installer process can exit", async (t) => {
  const server = http.createServer((req, res) => {
    if (req.url === "/redirect") {
      res.writeHead(302, { Location: "/archive", "Content-Length": "8" });
      res.end("redirect");
      return;
    }
    res.writeHead(200, { "Content-Length": "7" });
    res.end("payload");
  });
  server.keepAliveTimeout = 30_000;
  await new Promise((resolve, reject) => {
    server.once("error", reject);
    server.listen(0, "127.0.0.1", resolve);
  });
  t.after(() => {
    server.closeAllConnections();
    server.close();
  });

  const url = `http://127.0.0.1:${server.address().port}/redirect`;
  const script = `
    const http = require("node:http");
    http.globalAgent = new http.Agent({ keepAlive: true });
    require(${JSON.stringify(require.resolve("./install.js"))}).fetch(${JSON.stringify(url)})
      .then((data) => console.log(data.toString()))
      .catch((err) => { console.error(err); process.exitCode = 1; });
  `;
  const { stdout } = await promisify(execFile)(process.execPath, ["-e", script], {
    // This test exercises local HTTP keep-alive, independent of any host proxy.
    env: { ...process.env, NODE_OPTIONS: "", NODE_USE_ENV_PROXY: "0", NO_PROXY: "127.0.0.1" },
    timeout: 5_000,
  });
  assert.equal(stdout, "payload\n");
});

test("parseChecksumManifest selects one exact asset entry", () => {
  const manifest = [
    `${"1".repeat(64)}  cc-connect-next-v0.1.0-beta.1-linux-amd64.tar.gz`,
    `${digest}  ${filename}`,
    `${"2".repeat(64)} *cc-connect-next-v0.1.0-beta.1-windows-amd64.zip`,
  ].join("\n");

  assert.equal(parseChecksumManifest(manifest, filename), digest);
});

test("parseChecksumManifest fails closed for missing, duplicate, or malformed entries", () => {
  assert.throws(
    () => parseChecksumManifest(`${digest}  another-file.tar.gz\n`, filename),
    /does not contain/
  );
  assert.throws(
    () => parseChecksumManifest(`${digest}  ${filename}\n${digest} *${filename}\n`, filename),
    /more than one/
  );
  assert.throws(
    () => parseChecksumManifest(`not-a-sha256  ${filename}\n`, filename),
    /does not contain/
  );
});

test("verifyChecksum accepts matching bytes and rejects a mismatch", () => {
  assert.equal(verifyChecksum(payload, digest, filename), digest);
  assert.throws(
    () => verifyChecksum(Buffer.from("tampered"), digest, filename),
    /checksum mismatch/
  );
  assert.throws(
    () => verifyChecksum(payload, "abc", filename),
    /invalid SHA-256/
  );
});
