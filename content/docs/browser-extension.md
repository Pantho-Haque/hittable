---
title: Browser extension
description: Hittable Companion is a small Chrome extension that lets the web app reach servers on your own machine. What it does, how to install it, and how to make it work with a self-hosted instance.
---

## Why it exists

The web app sends remote requests through a server-side proxy, which is what sidesteps browser CORS rules. A server cannot reach `http://localhost:3000` on *your* machine, though. The extension's background service worker runs in your browser, is not bound by page CORS, and can. So:

| URL | Path | Needs the extension |
| :--- | :--- | :--- |
| `http://localhost:…`, `http://127.0.0.1:…` | the extension's service worker, from your machine | yes |
| anything else | the app's `/api/proxy` | no |

Without the extension, sending to localhost fails with **Install the Hittable Extension to make localhost requests**, and a **No Extension Found** dialog explains how to install it. The extension also returns the cookies for the target origin with each response, which the proxy cannot do.

The extension is deliberately narrow: its host permissions cover only `http://localhost/*` and `http://127.0.0.1/*`. It is not a general CORS bypass, and remote requests never touch it. `0.0.0.0` is not covered; use `127.0.0.1`.

## Install

The extension is distributed as a zip attached to each `v*` release and loaded unpacked. It is Chrome / Chromium only (Manifest V3).

1. Download `hittable-extension.zip` from the [latest release](https://github.com/Pantho-Haque/hittable/releases/latest/download/hittable-extension.zip) and unzip it.
2. Open `chrome://extensions`.
3. Turn on **Developer mode** (top right).
4. Click **Load unpacked** and select the unzipped folder.

Reload the Hittable tab. The app pings for the extension every half second for about five seconds after load; once it answers, localhost requests go through it. There is no indicator to watch for: the absence of the **No Extension Found** dialog is the confirmation.

## How it talks to the page

Three messages, all through `window.postMessage`:

| Message | Direction | Meaning |
| :--- | :--- | :--- |
| `HITTABLE_PING` | app → extension | is anyone there? |
| `HITTABLE_EXTENSION_READY` | extension → app | yes |
| `HITTABLE_REQUEST` / `HITTABLE_RESPONSE` | app → extension → app | `{ url, method, headers, body }` in; `{ success, data, status, statusText, ok, headers, cookies }` or `{ success: false, error }` out |

The content script relays `HITTABLE_REQUEST` to the service worker, which performs the `fetch` (omitting the body for `GET` and `HEAD`), parses JSON when the content type says so, flattens the headers, reads the origin's cookies and replies. The app gives up after 10 seconds with **Extension request timed out**.

## Self-hosted instances

The content script is only injected on the origins listed in the manifest:

```json
"content_scripts": [
  { "matches": ["https://hittable.vercel.app/*", "http://localhost:3000/*"], "js": ["content.js"] }
]
```

A Hittable you host on any other origin will never see the extension, and localhost requests will fail there. Edit `browserExtension/manifest.json`, add your origin (for example `"https://api-tools.example.com/*"`), and reload the unpacked extension. There is nothing else to configure.

## Permissions it asks for

| Permission | Used for |
| :--- | :--- |
| `host_permissions` for `localhost` and `127.0.0.1` | making the requests |
| `cookies` | returning the cookies of the target origin with the response |
| `declarativeNetRequest` | declared but currently unused |

The extension has no UI, no storage and no network activity of its own beyond the requests you send.
