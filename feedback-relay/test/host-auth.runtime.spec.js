import {describe, expect, it} from "vitest";

import {createGitHubAppJWT, installationAccessToken} from "../src/github-app.js";

function privateKeyPEM(bytes) {
  const encoded = btoa(String.fromCharCode(...new Uint8Array(bytes)));
  return [
    "-----BEGIN PRIVATE KEY-----",
    ...encoded.match(/.{1,64}/g),
    "-----END PRIVATE KEY-----",
  ].join("\n");
}

describe("GitHub App auth in the Workers runtime", () => {
  it("imports PKCS#8 and signs an RS256 JWT with Web Crypto", async () => {
    const pair = await crypto.subtle.generateKey(
      {
        name: "RSASSA-PKCS1-v1_5",
        modulusLength: 2048,
        publicExponent: new Uint8Array([1, 0, 1]),
        hash: "SHA-256",
      },
      true,
      ["sign", "verify"],
    );
    const pem = privateKeyPEM(await crypto.subtle.exportKey("pkcs8", pair.privateKey));
    const jwt = await createGitHubAppJWT(
      {GITHUB_APP_ID: "123", GITHUB_APP_PRIVATE_KEY: pem},
      Date.parse("2026-08-31T12:00:00Z"),
    );
    const [header, payload, signature] = jwt.split(".");
    expect(JSON.parse(atob(header.replaceAll("-", "+").replaceAll("_", "/")))).toEqual({
      alg: "RS256",
      typ: "JWT",
    });
    expect(
      await crypto.subtle.verify(
        "RSASSA-PKCS1-v1_5",
        pair.publicKey,
        Uint8Array.from(atob(signature.replaceAll("-", "+").replaceAll("_", "/")), (value) =>
          value.charCodeAt(0),
        ),
        new TextEncoder().encode(`${header}.${payload}`),
      ),
    ).toBe(true);

    let outgoing;
    const token = await installationAccessToken({
      GITHUB_APP_ID: "123", GITHUB_APP_INSTALLATION_ID: "456", GITHUB_APP_PRIVATE_KEY: pem,
    }, "owner/repository", async (url, init) => {
      outgoing = new Request(url, init);
      return Response.json({token: "test-token", expires_at: new Date(Date.now() + 60_000).toISOString()});
    });
    expect(token).toBe("test-token");
    expect(outgoing.redirect).toBe("manual");

    await expect(installationAccessToken({
      GITHUB_APP_ID: "123", GITHUB_APP_INSTALLATION_ID: "456", GITHUB_APP_PRIVATE_KEY: pem,
    }, "owner/repository", async () => new Response(null, {
      status: 307, headers: {location: "https://untrusted.example/token"},
    }))).rejects.toMatchObject({code: "GitHub App token request rejected", status: 307});
  });
});
