---
title: Self-hosting
description: Run your own instance so request traffic and GitHub sign-in go through a server you control. One Next.js app, an optional GitHub OAuth app, and a few URLs to change.
---

## Why

In the hosted app, every request to a remote URL passes through `/api/proxy` on `hittable.vercel.app`, headers and body included, and GitHub sign-in uses that instance's OAuth app. Nothing is stored there, but if your API keys or your GitHub token should never pass through a server you do not run, host the app yourself. Localhost requests never use the proxy; they go through the [browser extension](/docs/browser-extension).

## What you are deploying

A single Next.js 16 application (React 19, Tailwind 4, pnpm). There is no database. Two groups of API routes:

- `/api/proxy`, the request proxy. It needs no configuration; its limits can be tuned with the optional settings in [The proxy](#the-proxy).
- `/api/github/*`, GitHub sign-in and the repository calls the workspace makes. These need a GitHub OAuth app and three environment variables, below. Without them sign-in is unavailable, and **Open a local folder** still works.

## GitHub sign-in

### Create the OAuth app

In GitHub, go to **Settings → Developer settings → OAuth Apps → New OAuth App** (for an organisation-owned app, the organisation's settings). Fill in:

| Field | Value |
| :--- | :--- |
| Homepage URL | `https://<host>` |
| Authorization callback URL | `https://<host>/api/github/callback` |

Create the app, copy the **Client ID**, and generate a **client secret**. For local development, a second app with `http://localhost:3000` and `http://localhost:3000/api/github/callback` keeps production untouched.

### Environment variables

| Variable | Required | Meaning |
| :--- | :--- | :--- |
| `GITHUB_CLIENT_ID` | yes | the OAuth app's client ID |
| `GITHUB_CLIENT_SECRET` | yes | the OAuth app's client secret; it never leaves the server |
| `SESSION_SECRET` | yes | at least 32 bytes; encrypts the session cookie. Generate one with `openssl rand -base64 32` |
| `APP_URL` | no | the public origin, for example `https://api-tools.example.com`, when a reverse proxy rewrites the `Host` header. Used to build the callback URL |
| `GITHUB_API_URL`, `GITHUB_OAUTH_URL`, `GITHUB_GRAPHQL_URL` | no, tests only | point the app at a fake GitHub. Default `https://api.github.com`, `https://github.com` and `<api>/graphql` |

If any required variable is missing, or `SESSION_SECRET` is shorter than 32 bytes, `/api/github/session` returns a `config` error that names the variable (never its value) and sign-in is not offered.

### How it works

The app runs the OAuth web flow itself, with `state` and PKCE (S256), and asks for the scopes `repo read:user`. After the callback, the token is stored only in an AES-256-GCM encrypted, `HttpOnly`, `SameSite=Lax` cookie (`Secure` over HTTPS). Every GitHub call goes through `/api/github/*`, and mutating calls must come from the app's own origin. Sign-out revokes the grant on GitHub, then clears the cookie. Rotating `SESSION_SECRET` signs everyone out.

## Vercel

Fork the repository and import it, or use the one-click link:

```
https://vercel.com/new/clone?repository-url=https://github.com/Pantho-Haque/hittable
```

The build uses the defaults (`pnpm install`, `next build`). Add the GitHub variables under the project's environment variables. `.vercelignore` already excludes the terminal app, the marketing project and the code-graph index so uploads stay small.

## Any Node host

```sh
git clone https://github.com/Pantho-Haque/hittable
cd hittable
pnpm install
pnpm build
GITHUB_CLIENT_ID=… GITHUB_CLIENT_SECRET=… SESSION_SECRET=… pnpm start   # listens on :3000
```

Node 22.19 or newer. Put it behind your usual reverse proxy with TLS, and set `APP_URL` if the proxy does not pass the original `Host`.

## Docker, for development

The repository's Docker setup is a **development** environment, not a production image: `docker-compose.yml` uses the stock Node image, bind-mounts the repository, runs `pnpm install` and `pnpm dev` with ports `3000` (app) and `9229` (Node inspector). The wrapper does the checks and cleanup for you:

```sh
./run.sh
```

## Docker, for production

The `Dockerfile` builds a small production image: a multi-stage build that ships Next.js's standalone server, runs as the unprivileged `node` user and has a health check.

```sh
docker build -t hittable .
docker run -p 3000:3000 \
  -e GITHUB_CLIENT_ID=… -e GITHUB_CLIENT_SECRET=… -e SESSION_SECRET=… \
  hittable
# reaching APIs on your own network:
docker run -p 3000:3000 -e PROXY_ALLOW_PRIVATE_NETWORKS=true hittable
```

## Things to change for your domain

| Where | What |
| :--- | :--- |
| GitHub OAuth app | Homepage and callback URL on your host, see [Create the OAuth app](#create-the-oauth-app) |
| `app/layout.tsx` | `metadataBase` is `https://hittable.vercel.app`; used for canonical and Open Graph URLs |
| `app/sitemap.ts`, `app/robots.ts` | the same origin, hard-coded |
| `browserExtension/manifest.json` | add your origin to `content_scripts.matches`, otherwise the extension never activates on your instance and localhost requests fail. See [Browser extension](/docs/browser-extension#self-hosted-instances) |
| `next.config.ts` | `images.remotePatterns` if you serve images from elsewhere |

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

1. Deploy and open `/hittable`.
2. Sign in with GitHub, pick a repository and open a `.hit` file. If sign-in is not offered, check the [environment variables](#environment-variables).
3. Send a request to a public URL. It should go through your `/api/proxy`.
4. In Chrome or Edge, **Open a local folder** and check the files appear.
5. Install the extension with your origin added to its manifest, reload, and send a request to `http://127.0.0.1:<port>`.
6. Check `/robots.txt` and `/sitemap.xml` show your domain.
