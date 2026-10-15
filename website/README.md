# CC Connect Next website

The independent public website lives here. It is separate from the local management UI in `web/` and never connects to a running daemon. Chinese and English content, the message-flow example, and command-copy controls run entirely in the browser. Only the language preference is stored locally.

```sh
cd website
npm ci
npm run check
npm run dev -- --ip 127.0.0.1 --port 4175
```

The site uses Cloudflare Workers static assets, with no application worker, account system, database, or secrets. `npm run check` verifies assets, links, translations, command syntax, and a deployment dry run. Browser QA covers responsive layouts and interactive controls.

The prepared production target is `cc-connect-next.timmyagentic.si`, matching this project's existing domain family. Custom-domain routing, canonical URLs, and the sitemap use that hostname. It is not provisioned or published by local checks. After explicit production authorization, verify the hostname is available in the intended Cloudflare account, run `npm run deploy`, verify the public routes, and only then add the live URL to the GitHub repository homepage.

`/docs` provides the three main setup paths and links to the authoritative repository guides. The hero and demo do not claim identical capabilities across agents and platforms. Migration copy offers only the read-only dry run; the installation guide documents the explicit service-switch operation.

Icons are from Tabler Icons 3.49.0, with their MIT license under `public/icons/LICENSE.txt`. The decorative landscape is generated for this website family. The message flow is a labeled interactive example, not a captured customer conversation.
