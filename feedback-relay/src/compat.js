import { fetchHandler } from "./relay.js";

/** @typedef {Env & {GITHUB_TOKEN: string}} RelayEnv */

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

/** @param {Request} request @param {RelayEnv} env @returns {Promise<Response>} */
async function compatibilityHandler(request, env) {
  const url = new URL(request.url);
  if (request.method !== "POST" || url.pathname !== "/v1/feedback" || url.search !== "") {
    return fetchHandler(request, env);
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
    return fetchHandler(rebuiltRequest(request, raw), env);
  }
  const translated = translateLegacy(decoded);
  const body = JSON.stringify(translated ?? decoded);
  return fetchHandler(rebuiltRequest(request, body), env);
}

const worker = {fetch: compatibilityHandler};

export default worker;
export const _test = {translateLegacy, truncateUTF8};
