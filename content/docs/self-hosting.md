---
title: Self-hosting
description: Run your own instance so request traffic goes through a proxy you control. Zero configuration, one Next.js app, a few URLs to change.
---

## Why

In the hosted app, every request to a remote URL passes through `/api/proxy` on `hittable.vercel.app`, headers and body included. Nothing is stored there, but if your API keys should never transit a server you do not run, host the app yourself. Localhost requests never use the proxy at all; they go through the [browser extension](/docs/browser-extension).

## What you are deploying

A single Next.js 16 application (React 19, Tailwind 4, pnpm). It needs **no environment variables**: there is no database, no auth provider, no API key. The two API routes are the request proxy and an asset proxy for the HTML preview.

## Vercel

The quickest path. Fork the repository and import it, or use the one-click link:

```
https://vercel.com/new/clone?repository-url=https://github.com/Pantho-Haque/hittable
```

The build uses the defaults (`pnpm install`, `next build`). `.vercelignore` already excludes the terminal app, the video project and the code-graph notes so uploads stay small.

## Any Node host

```sh
git clone https://github.com/Pantho-Haque/hittable
cd hittable
pnpm install
pnpm build
pnpm start          # listens on :3000
```

Node 20 or newer. Put it behind your usual reverse proxy with TLS.

## Docker, for development

The repository's Docker setup is a **development** environment, not a production image: the `Dockerfile` installs Node and pnpm only, and `docker-compose.yml` bind-mounts the repository, runs `pnpm install` and `pnpm dev` with ports `3000` (app) and `9229` (Node inspector). The wrapper does the checks and cleanup for you:

```sh
./run.sh
```

For production, write a standard multi-stage Next.js image or use a Node host as above.

## Things to change for your domain

| Where | What |
| :--- | :--- |
| `app/layout.tsx` | `metadataBase` is `https://hittable.vercel.app`; used for canonical and Open Graph URLs |
| `app/sitemap.ts`, `app/robots.ts` | the same origin, hard-coded |
| `browserExtension/manifest.json` | add your origin to `content_scripts.matches`, otherwise the extension never activates on your instance and localhost requests fail. See [Browser extension](/docs/browser-extension#self-hosted-instances) |
| `next.config.ts` | `images.remotePatterns` if you serve images from elsewhere |

The landing page fetches a résumé block from the author's portfolio API with a static fallback; it is cosmetic and safe to remove.

## The proxy, honestly

`POST /api/proxy` takes `{ url, method, headers, body }`, strips hop-by-hop headers, forwards the request and returns `{ data, status, statusText, ok, headers, cookies }`. A network failure still returns HTTP 200 with `status: 0` and an `error` field, so the client always gets a structured answer. `GET /api/proxy/asset?url=…` fetches images and stylesheets for the HTML preview with a 15-second timeout.

Both routes are **unauthenticated and have no allow-list or rate limit**. A publicly reachable instance is therefore an open HTTP proxy. If you expose one, put it behind something that identifies your users (a VPN, an identity-aware proxy, basic auth at the reverse proxy), or restrict the origins it will fetch. Also note that `set-cookie` headers are split on commas, which mis-splits cookies whose `Expires` attribute contains one.

## Checklist

1. Deploy, open `/hittable`, switch to Directory mode and pick a folder.
2. Send a request to a public URL. It should go through your `/api/proxy`.
3. Install the extension with your origin added to its manifest, reload, and send a request to `http://127.0.0.1:<port>`.
4. Check `/robots.txt` and `/sitemap.xml` show your domain.
