/**
 * Host-owned GitHub App authentication for the CC Connect feedback Relay.
 *
 * GitHub downloads PKCS#1 private keys. Convert the key to unencrypted PKCS#8
 * before storing it as GITHUB_APP_PRIVATE_KEY so Workers Web Crypto can import
 * it without a Node.js crypto dependency.
 */

/** @typedef {Env & {GITHUB_APP_PRIVATE_KEY: string}} GitHubAppEnv */

const GITHUB_API_VERSION = "2026-03-10";
const GITHUB_TIMEOUT_MS = 15_000;
const MAX_AUTH_RESPONSE_BYTES = 64 * 1024;
const MAX_PRIVATE_KEY_BYTES = 32 * 1024;
const MAX_TOKEN_BYTES = 4096;

export class GitHubAppAuthError extends Error {
  /** @param {string} code @param {number | undefined} [status] */
  constructor(code, status) {
    super(code);
    this.name = "GitHubAppAuthError";
    this.code = code;
    this.status = status;
  }
}

/** @param {unknown} value @returns {value is Record<string, unknown>} */
function isObject(value) {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}

/** @param {unknown} value @param {string} name @returns {string} */
function positiveInteger(value, name) {
  const text = typeof value === "string" ? value.trim() : "";
  if (!/^[1-9][0-9]{0,19}$/.test(text)) {
    throw new GitHubAppAuthError(`invalid ${name}`);
  }
  return text;
}

/** @param {unknown} value @returns {string} */
function repositoryName(value) {
  if (typeof value !== "string" || !/^[A-Za-z0-9_.-]+\/[A-Za-z0-9_.-]+$/.test(value)) {
    throw new GitHubAppAuthError("invalid GitHub repository");
  }
  const [owner, repository] = value.split("/");
  if (owner === "." || owner === ".." || repository === "." || repository === "..") {
    throw new GitHubAppAuthError("invalid GitHub repository");
  }
  return repository;
}

/** @param {unknown} value @returns {ArrayBuffer} */
function pkcs8Bytes(value) {
  if (typeof value !== "string" || new TextEncoder().encode(value).byteLength > MAX_PRIVATE_KEY_BYTES) {
    throw new GitHubAppAuthError("invalid GitHub App private key");
  }
  const normalized = value.trim().replaceAll("\r\n", "\n");
  const match = /^-----BEGIN PRIVATE KEY-----\n([A-Za-z0-9+/=\n]+)\n-----END PRIVATE KEY-----$/.exec(normalized);
  if (!match) {
    throw new GitHubAppAuthError("GitHub App private key must be unencrypted PKCS#8 PEM");
  }
  const encoded = match[1].replaceAll("\n", "");
  if (encoded === "" || encoded.length % 4 !== 0) {
    throw new GitHubAppAuthError("invalid GitHub App private key");
  }
  try {
    const binary = atob(encoded);
    const bytes = new Uint8Array(binary.length);
    for (let index = 0; index < binary.length; index += 1) {
      bytes[index] = binary.charCodeAt(index);
    }
    return bytes.buffer;
  } catch {
    throw new GitHubAppAuthError("invalid GitHub App private key");
  }
}

/** @param {Uint8Array} bytes @returns {string} */
function base64URL(bytes) {
  let binary = "";
  for (const byte of bytes) {
    binary += String.fromCharCode(byte);
  }
  return btoa(binary).replaceAll("+", "-").replaceAll("/", "_").replace(/=+$/, "");
}

/** @param {unknown} value @returns {string} */
function base64URLJSON(value) {
  return base64URL(new TextEncoder().encode(JSON.stringify(value)));
}

/**
 * @param {GitHubAppEnv} env
 * @param {number} [nowMilliseconds]
 * @returns {Promise<string>}
 */
export async function createGitHubAppJWT(env, nowMilliseconds = Date.now()) {
  const appID = positiveInteger(env.GITHUB_APP_ID, "GitHub App ID");
  const now = Math.floor(nowMilliseconds / 1000);
  const header = base64URLJSON({alg: "RS256", typ: "JWT"});
  const payload = base64URLJSON({iat: now - 60, exp: now + 540, iss: appID});
  const signingInput = `${header}.${payload}`;
  let key;
  try {
    key = await crypto.subtle.importKey(
      "pkcs8",
      pkcs8Bytes(env.GITHUB_APP_PRIVATE_KEY),
      {name: "RSASSA-PKCS1-v1_5", hash: "SHA-256"},
      false,
      ["sign"],
    );
  } catch (error) {
    if (error instanceof GitHubAppAuthError) throw error;
    throw new GitHubAppAuthError("GitHub App private key import failed");
  }
  let signature;
  try {
    signature = await crypto.subtle.sign(
      "RSASSA-PKCS1-v1_5",
      key,
      new TextEncoder().encode(signingInput),
    );
  } catch {
    throw new GitHubAppAuthError("GitHub App JWT signing failed");
  }
  return `${signingInput}.${base64URL(new Uint8Array(signature))}`;
}

/** @param {Response} response @returns {Promise<unknown>} */
async function readBoundedJSON(response) {
  const declared = Number(response.headers.get("content-length"));
  if (Number.isFinite(declared) && declared > MAX_AUTH_RESPONSE_BYTES) {
    throw new GitHubAppAuthError("GitHub App token response is too large", response.status);
  }
  if (!response.body) {
    throw new GitHubAppAuthError("GitHub App token response is empty", response.status);
  }
  const reader = response.body.getReader();
  const chunks = [];
  let total = 0;
  for (;;) {
    const {done, value} = await reader.read();
    if (done) break;
    total += value.byteLength;
    if (total > MAX_AUTH_RESPONSE_BYTES) {
      await reader.cancel();
      throw new GitHubAppAuthError("GitHub App token response is too large", response.status);
    }
    chunks.push(value);
  }
  const bytes = new Uint8Array(total);
  let offset = 0;
  for (const chunk of chunks) {
    bytes.set(chunk, offset);
    offset += chunk.byteLength;
  }
  try {
    return JSON.parse(new TextDecoder("utf-8", {fatal: true}).decode(bytes));
  } catch {
    throw new GitHubAppAuthError("GitHub App token response is invalid", response.status);
  }
}

/**
 * @param {GitHubAppEnv} env
 * @param {string} repository
 * @param {typeof fetch} [fetcher]
 * @param {number} [nowMilliseconds]
 * @returns {Promise<string>}
 */
export async function installationAccessToken(
  env,
  repository,
  fetcher = fetch,
  nowMilliseconds = Date.now(),
) {
  const installationID = positiveInteger(env.GITHUB_APP_INSTALLATION_ID, "GitHub App installation ID");
  const targetRepository = repositoryName(repository);
  const jwt = await createGitHubAppJWT(env, nowMilliseconds);
  let response;
  try {
    response = await fetcher(
      `https://api.github.com/app/installations/${installationID}/access_tokens`,
      {
        method: "POST",
        signal: AbortSignal.timeout(GITHUB_TIMEOUT_MS),
        headers: {
          authorization: `Bearer ${jwt}`,
          accept: "application/vnd.github+json",
          "content-type": "application/json",
          "user-agent": "cc-connect-feedback-relay",
          "x-github-api-version": GITHUB_API_VERSION,
        },
        body: JSON.stringify({
          repositories: [targetRepository],
          permissions: {issues: "write"},
        }),
      },
    );
  } catch {
    throw new GitHubAppAuthError("GitHub App token request failed");
  }
  if (!response.ok) {
    if (response.body) {
      try {
        await response.body.cancel();
      } catch {
        // The status is authoritative; cancellation failure must not change it.
      }
    }
    throw new GitHubAppAuthError("GitHub App token request rejected", response.status);
  }
  const decoded = await readBoundedJSON(response);
  const token = isObject(decoded) ? decoded.token : undefined;
  const expiresAt = isObject(decoded) ? decoded.expires_at : undefined;
  if (
    typeof token !== "string" ||
    token === "" ||
    new TextEncoder().encode(token).byteLength > MAX_TOKEN_BYTES ||
    /\s|[\u0000-\u001f\u007f]/.test(token) ||
    typeof expiresAt !== "string" ||
    !Number.isFinite(Date.parse(expiresAt)) ||
    Date.parse(expiresAt) <= nowMilliseconds
  ) {
    throw new GitHubAppAuthError("GitHub App token response is invalid", response.status);
  }
  return token;
}

export const _test = {base64URL, pkcs8Bytes, repositoryName};
