import {GitHubAppAuthError, installationAccessToken} from "./github-app.js";
import {fetchHandler, _test as relayTest} from "./relay.js";

/** @typedef {Env & {GITHUB_APP_PRIVATE_KEY: string}} RelayEnv */

const MAX_REQUEST_BYTES = 96 * 1024;
const MAX_DESCRIPTION_BYTES = 4_000;
const MAX_METADATA_BYTES = 160;
const LEGACY_FIELDS = new Set([
  "schema",
  "install_id",
  "version",
  "os",
  "arch",
  "agent",
  "title",
  "body",
]);

class RequestTooLargeError extends Error {}

/** @param {number} status @param {unknown} payload @returns {Response} */
function json(status, payload) {
  return new Response(JSON.stringify(payload), {
    status,
    headers: {
      "content-type": "application/json; charset=utf-8",
      "cache-control": "no-store",
      "content-security-policy": "default-src 'none'; frame-ancestors 'none'",
      "x-content-type-options": "nosniff",
    },
  });
}

/** @param {Request} request @returns {Promise<Uint8Array>} */
async function readBoundedBody(request) {
  const declared = Number(request.headers.get("content-length"));
  if (Number.isFinite(declared) && declared > MAX_REQUEST_BYTES) {
    throw new RequestTooLargeError();
  }
  if (!request.body) {
    return new Uint8Array();
  }
  const reader = request.body.getReader();
  const chunks = [];
  let total = 0;
  while (true) {
    const {done, value} = await reader.read();
    if (done) break;
    total += value.byteLength;
    if (total > MAX_REQUEST_BYTES) {
      await reader.cancel();
      throw new RequestTooLargeError();
    }
    chunks.push(value);
  }
  const result = new Uint8Array(total);
  let offset = 0;
  for (const chunk of chunks) {
    result.set(chunk, offset);
    offset += chunk.byteLength;
  }
  return result;
}

/** @param {string} value @param {number} maximum @returns {string} */
function truncateUTF8(value, maximum) {
  const encoder = new TextEncoder();
  if (encoder.encode(value).byteLength <= maximum) {
    return value;
  }
  let result = "";
  for (const character of value) {
    if (encoder.encode(result + character).byteLength > maximum) break;
    result += character;
  }
  return result;
}

/** @param {unknown} value @returns {string} */
function metadata(value) {
  if (typeof value !== "string") return "";
  return truncateUTF8(value.replace(/[\u0000-\u001f\u007f]/g, " ").trim(), MAX_METADATA_BYTES);
}

/** @param {string} value @returns {string} */
function reportText(value) {
  return value.replace(/[\u0000-\u0008\u000b-\u001f\u007f]/g, " ").trim();
}

/** @param {unknown} value @returns {value is Record<string, unknown>} */
function isObject(value) {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}

/** @param {unknown} value @returns {Record<string, unknown> | null} */
function translateLegacy(value) {
  if (
    !isObject(value) ||
    value.schema !== 1 ||
    typeof value.title !== "string" ||
    typeof value.body !== "string" ||
    value.title.trim() === "" ||
    value.body.trim() === "" ||
    Object.keys(value).some((field) => !LEGACY_FIELDS.has(field))
  ) {
    return null;
  }
  const description = truncateUTF8(
    `${reportText(value.title)}\n\n${reportText(value.body)}`,
    MAX_DESCRIPTION_BYTES,
  );
  return {
    schema: 1,
    user_approved: true,
    environment: {
      product: "cc-connect-next",
      ...(metadata(value.version) && {version: metadata(value.version)}),
      ...(metadata(value.os) && {os: metadata(value.os)}),
      ...(metadata(value.arch) && {arch: metadata(value.arch)}),
      ...(metadata(value.agent) && {agent: metadata(value.agent)}),
    },
    description,
  };
}

/** @param {Request} request @param {string} body @returns {Request} */
function rebuiltRequest(request, body) {
  const headers = new Headers(request.headers);
  headers.delete("content-length");
  return new Request(request.url, {
    method: request.method,
    headers,
    body,
    signal: request.signal,
  });
}

/** @param {RelayEnv} env @returns {string | null} */
function configurationError(env) {
  const repositoryParts = typeof env.GITHUB_REPO === "string" ? env.GITHUB_REPO.split("/") : [];
  if (
    typeof env.GITHUB_APP_ID !== "string" ||
    !/^[1-9][0-9]{0,19}$/.test(env.GITHUB_APP_ID.trim()) ||
    typeof env.GITHUB_APP_INSTALLATION_ID !== "string" ||
    !/^[1-9][0-9]{0,19}$/.test(env.GITHUB_APP_INSTALLATION_ID.trim()) ||
    typeof env.GITHUB_APP_PRIVATE_KEY !== "string" ||
    !env.GITHUB_APP_PRIVATE_KEY.trim().startsWith("-----BEGIN PRIVATE KEY-----") ||
    typeof env.GITHUB_REPO !== "string" ||
    !/^[A-Za-z0-9_.-]+\/[A-Za-z0-9_.-]+$/.test(env.GITHUB_REPO) ||
    repositoryParts.some((part) => part === "." || part === "..") ||
    (env.GITHUB_LABEL !== undefined &&
      (typeof env.GITHUB_LABEL !== "string" ||
        !/^[A-Za-z0-9][A-Za-z0-9 ._:-]{0,49}$/.test(env.GITHUB_LABEL))) ||
    !env.RATE_LIMITER ||
    typeof env.RATE_LIMITER.limit !== "function"
  ) {
    return "relay is not configured";
  }
  return null;
}

/** @param {Request} request @returns {string} */
function clientRateLimitKey(request) {
  const connectingIP = request.headers.get("cf-connecting-ip")?.trim().slice(0, 128);
  return connectingIP ? `ip:${connectingIP}` : "unknown";
}

/** @param {RelayEnv} env @param {string} token @param {string} acceptedKey */
function authorizedRelayEnv(env, token, acceptedKey) {
  const rateLimiter = {
    /** @param {{key: string}} input */
    async limit({key}) {
      return {success: key === acceptedKey};
    },
  };
  return {
    ...env,
    GITHUB_TOKEN: token,
    RATE_LIMITER: rateLimiter,
  };
}

/** @param {Request} request @param {RelayEnv} env @returns {Promise<Response>} */
function rejectWithoutAuthentication(request, env) {
  return fetchHandler(request, {...env, GITHUB_TOKEN: "invalid-request-placeholder"});
}

/** @param {Request} request @param {RelayEnv} env @returns {Promise<Response>} */
async function compatibilityHandler(request, env) {
  const url = new URL(request.url);
  if (request.method !== "POST" || url.pathname !== "/v1/feedback" || url.search !== "") {
    return rejectWithoutAuthentication(request, env);
  }
  const contentType = request.headers.get("content-type") || "";
  if (!/^application\/json(?:\s*;|$)/i.test(contentType)) {
    return rejectWithoutAuthentication(request, env);
  }
  let bytes;
  try {
    bytes = await readBoundedBody(request);
  } catch (error) {
    if (error instanceof RequestTooLargeError) {
      return json(413, {error: "request is too large"});
    }
    return json(400, {error: "invalid JSON"});
  }
  const raw = new TextDecoder().decode(bytes);
  let decoded;
  try {
    decoded = JSON.parse(raw);
  } catch {
    return rejectWithoutAuthentication(rebuiltRequest(request, raw), env);
  }
  const translated = translateLegacy(decoded);
  const submission = translated ?? decoded;
  const body = JSON.stringify(submission);
  const delegatedRequest = rebuiltRequest(request, body);
  if (relayTest.validateSubmission(submission)) {
    return rejectWithoutAuthentication(delegatedRequest, env);
  }
  if (configurationError(env)) {
    return json(500, {error: "relay is not configured"});
  }

  const rateLimitKey = clientRateLimitKey(request);
  let rateLimit;
  try {
    rateLimit = await env.RATE_LIMITER.limit({key: rateLimitKey});
  } catch {
    console.error(JSON.stringify({
      message: "feedback rate limit failed",
      error: "rate limiter unavailable",
    }));
    return json(500, {error: "internal relay error"});
  }
  if (!rateLimit.success) {
    return json(429, {error: "rate limited"});
  }

  let token;
  try {
    token = await installationAccessToken(env, env.GITHUB_REPO);
  } catch (error) {
    console.error(JSON.stringify({
      message: "GitHub App authentication failed",
      code: error instanceof GitHubAppAuthError ? error.code : "unexpected authentication failure",
      status: error instanceof GitHubAppAuthError ? error.status : undefined,
    }));
    return json(502, {error: "github app authentication failed"});
  }
  return fetchHandler(delegatedRequest, authorizedRelayEnv(env, token, rateLimitKey));
}

const worker = {fetch: compatibilityHandler};

export default worker;
export const _test = {translateLegacy, truncateUTF8};
