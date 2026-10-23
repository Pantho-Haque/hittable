---
title: Self-hosting
description: Run your own instance so request traffic goes through a proxy you control. Zero configuration, one Next.js app, a few URLs to change.
---

## Why

In the hosted app, every request to a remote URL passes through `/api/proxy` on `hittable.vercel.app`, headers and body included. Nothing is stored there, but if your API keys should never transit a server you do not run, host the app yourself. Localhost requests never use the proxy at all; they go through the [browser extension](/docs/browser-extension).

## What you are deploying

A single Next.js 16 application (React 19, Tailwind 4, pnpm). It needs **no environment variables**: there is no database, no auth provider, no API key. The one API route is the request proxy; its limits can be tuned with the optional settings in [The proxy](#the-proxy).

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

Node 22.19 or newer. Put it behind your usual reverse proxy with TLS.

## Docker, for development

The repository's Docker setup is a **development** environment, not a production image: `docker-compose.yml` uses the stock Node image, bind-mounts the repository, runs `pnpm install` and `pnpm dev` with ports `3000` (app) and `9229` (Node inspector). The wrapper does the checks and cleanup for you:

```sh
./run.sh
```

## Docker, for production

The `Dockerfile` builds a small production image: a multi-stage build that ships Next.js's standalone server, runs as the unprivileged `node` user and has a health check.

```sh
docker build -t hittable .
docker run -p 3000:3000 hittable
# reaching APIs on your own network:
docker run -p 3000:3000 -e PROXY_ALLOW_PRIVATE_NETWORKS=true hittable
```

## Things to change for your domain

| Where | What |
| :--- | :--- |
| `app/layout.tsx` | `metadataBase` is `https://hittable.vercel.app`; used for canonical and Open Graph URLs |
| `app/sitemap.ts`, `app/robots.ts` | the same origin, hard-coded |
| `browserExtension/manifest.json` | add your origin to `content_scripts.matches`, otherwise the extension never activates on your instance and localhost requests fail. See [Browser extension](/docs/browser-extension#self-hosted-instances) |
| `next.config.ts` | `images.remotePatterns` if you serve images from elsewhere |

The landing page fetches a résumé block from the author's portfolio API with a static fallback; it is cosmetic and safe to remove.

## The proxy

`POST /api/proxy` takes `{ url, method, headers, body }`, strips hop-by-hop headers, forwards the request (following redirects, decompressing the body) and returns `{ data, status, statusText, ok, headers, cookies }`. Every `Set-Cookie` is kept, including ones whose `Expires` date contains a comma. Errors come back in the same shape with `status: 0` and an `error` message, under a meaningful HTTP status: `400` bad input, `403` blocked destination, `413` request too large, `429` rate limited (with `Retry-After`), `502` network error or response too large, `504` upstream timeout.

The route is unauthenticated, so it protects itself:

| Guard | Default | Setting |
| :--- | :--- | :--- |
| Private networks refused: loopback, `10/8`, `172.16/12`, `192.168/16`, link-local (cloud metadata), CGNAT, IPv6 ULA and link-local, IPv4-mapped addresses. Checked on the address actually dialled, so DNS rebinding and redirects into a private range are refused too | on | `PROXY_ALLOW_PRIVATE_NETWORKS=true` turns it off, for an instance that should reach internal APIs |
| Rate limit per client IP | 120 a minute | `PROXY_RATE_LIMIT_PER_MINUTE` (`0` = off) |
| Upstream timeout | 30 s | `PROXY_TIMEOUT_MS` |
| Largest response relayed | 4 MiB | `PROXY_MAX_RESPONSE_BYTES` |
| Largest request accepted | 4 MiB | `PROXY_MAX_REQUEST_BYTES` |

All are optional; `.env.example` lists them. The rate limit is kept in memory, so on a serverless host it applies per instance. For a hard global limit, add a rate-limit rule in front, such as a Vercel Firewall rule on `/api/proxy`.

## Checklist

1. Deploy, open `/hittable`, switch to Directory mode and pick a folder.
2. Send a request to a public URL. It should go through your `/api/proxy`.
3. Install the extension with your origin added to its manifest, reload, and send a request to `http://127.0.0.1:<port>`.
4. Check `/robots.txt` and `/sitemap.xml` show your domain.
