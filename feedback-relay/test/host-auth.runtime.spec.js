import {env} from "cloudflare:workers";
import {setupNetwork} from "@msw/cloudflare";
import {http, HttpResponse} from "msw";
import {afterAll, afterEach, beforeAll, describe, expect, it} from "vitest";

import {createGitHubAppJWT, installationAccessToken} from "../src/github-app.js";
import worker from "../src/compat.js";

const network = setupNetwork(http.all("*", () => { throw new Error("unexpected outbound request"); }));
beforeAll(() => network.enable());
afterEach(() => network.resetHandlers());
afterAll(() => network.disable());

function privateKeyPEM(bytes) {
  const encoded = btoa(String.fromCharCode(...new Uint8Array(bytes)));
  return [
    "-----BEGIN PRIVATE KEY-----",
    ...encoded.match(/.{1,64}/g),
    "-----END PRIVATE KEY-----",
  ].join("\n");
}

describe("GitHub App auth in the Workers runtime", () => {
  it("connects v2 App authentication to the actual durable object without duplicate issues", async () => {
    const pair = await crypto.subtle.generateKey({name: "RSASSA-PKCS1-v1_5", modulusLength: 2048, publicExponent: new Uint8Array([1, 0, 1]), hash: "SHA-256"}, true, ["sign", "verify"]);
    let authentications = 0, posts = 0, limits = 0;
    const configured = {...env,
      GITHUB_APP_ID: "123", GITHUB_APP_INSTALLATION_ID: "456",
      GITHUB_APP_PRIVATE_KEY: privateKeyPEM(await crypto.subtle.exportKey("pkcs8", pair.privateKey)),
      RATE_LIMITER: {async limit() { limits++; return {success: true}; }},
    };
    network.use(
      http.post("https://api.github.com/app/installations/456/access_tokens", async ({request}) => {
        authentications++;
        expect(await request.json()).toEqual({repositories: [env.GITHUB_REPO.split("/")[1]], permissions: {issues: "write"}});
        return HttpResponse.json({token: "runtime-installation-token", expires_at: new Date(Date.now() + 60000).toISOString()});
      }),
      http.post(`https://api.github.com/repos/${env.GITHUB_REPO}/issues`, async ({request}) => {
        posts++;
        expect(request.headers.get("authorization")).toBe("Bearer runtime-installation-token");
        expect((await request.json()).body).toContain("owned report");
        return HttpResponse.json({html_url: `https://github.com/${env.GITHUB_REPO}/issues/7`});
      }),
    );
    const value = {schema: 2, user_approved: true, report_id: crypto.randomUUID().replaceAll("-", ""), environment: {product: "cc-connect-next"}, description: "owned report"};
    const request = body => new Request("https://relay.example/v2/feedback", {method: "POST", headers: {"content-type": "application/json"}, body: JSON.stringify(body)});
    expect((await worker.fetch(request({...value, user_approved: false}), configured)).status).toBe(400);
    expect(authentications).toBe(0);
    expect(limits).toBe(0);
    expect((await worker.fetch(request(value), configured)).status).toBe(200);
    expect((await (await worker.fetch(request(value), configured)).json()).deduplicated).toBe(true);
    expect(authentications).toBe(2);
    expect(limits).toBe(2);
    expect(posts).toBe(1);
  });

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
