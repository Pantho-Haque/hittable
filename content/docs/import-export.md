---
title: Import and export
description: Move collections between Hittable, Postman and Insomnia. What maps, how variables translate, and what has no equivalent and is dropped.
---

Both apps speak the same three formats:

| Format | Detected by | Extension |
| :--- | :--- | :--- |
| Hittable native | a compressed, URL-safe base64 string (web only) or a `collectionName` + `items` object | `.txt` |
| Postman Collection v2.1 | `info.schema` containing `getpostman.com` | `.postman_collection.json` |
| Insomnia export v4 | `_type: "export"` with a `resources` array, or a bare array of `_type` resources | `.insomnia.json` |

## In the web app

**Import** (the import icon in the tool rail): paste the JSON or the Hittable string into the box and the format is detected. A collection whose name already exists is imported as `Name - 1`, `Name - 2` and so on. Every Postman import ends with a reminder that scripts and advanced auth types were skipped; every Insomnia import, that plugins, certificates and cookie jars were.

**Export**: a collection's `⋮` menu → **Export**, then pick **Hittable native** (copy the string), **Postman Collection v2.1** or **Insomnia export** (copy, or download the `.json`). Saved responses are stripped. Secrets are not resolved: they stay as variable references in the target syntax.

## In the terminal app

```sh
hittable -i collection.json          # import a Postman or Insomnia export, then open
hittable -e postman                  # export hittable/ as <folder>.postman_collection.json
hittable -e insomnia                 # export hittable/ as <folder>.insomnia.json
hittable -e postman -o api.json      # pick the file name
```

Import puts the collection under `hittable/<Collection name>/`, folders as directories, requests as `.hit` files, and merges variables into `hittable/env.json` without overwriting keys that already exist. It prints a report: requests, folders, variables, and everything that was dropped. Export walks `hittable/` recursively. The round trip is tested.

## Variable syntax

| Source | Syntax | Becomes |
| :--- | :--- | :--- |
| Postman | `{{varName}}` | `<<varName>>` |
| Insomnia | `{{ _.varName }}` | `<<varName>>` |
| Hittable → Postman | `<<KEY>>` | `{{KEY}}` |
| Hittable → Insomnia | `<<KEY>>` | `{{ _.KEY }}` |

Collection, folder and environment variables are collected into Hittable's flat variable map. Names keep their case.

## Postman mapping

| Postman | Hittable |
| :--- | :--- |
| `item` with `request` | a request (`.hit` file, or a route) |
| `item` with nested `item` | a folder |
| `url` as a string or as an object with `host`, `path`, `query` | `url` plus `params` from the query |
| `header[]` | `headers`, disabled rows skipped |
| body `raw` | `body` as-is |
| body `urlencoded` | a JSON object body |
| body `formdata` | key / value rows; file entries become `@filename` |
| body `file`, `graphql` | skipped |
| `auth` bearer, basic, apikey | the equivalent `Authorization` or custom header, with inheritance from folder and collection and `noauth` honoured |
| items marked `disabled: true` | skipped |

Dropped on import, and reported: pre-request and test scripts; OAuth1, OAuth2, AWS Signature v4, Digest, EdgeGrid, Hawk and NTLM auth; certificate, protocol-profile and proxy configuration. Dropped on export: notes, history, and auth presets (only the resulting headers are exported).

## Insomnia mapping

| Insomnia resource | Hittable |
| :--- | :--- |
| `workspace` | the collection |
| `request_group` | a folder, nested by `parentId` |
| `request` | a request; `parameters` become `params`, `headers` become `headers`, `body.text` or `body.params` become `body` |
| `environment` | variables, merged |

Dropped on import, and reported: plugin configuration, cookie jars, client certificates, WebSocket, gRPC and socket.io requests, mock routes, unit tests and API specs.

## Hittable native (web)

The string is the collection JSON run through `pako.deflate`, base64-encoded, then made URL-safe (`+` → `-`, `/` → `_`, padding removed). Importing accepts both the current nested `items` form and the older flat `curls` form, which is upgraded on the way in. A corrupt string fails with **Invalid or corrupted import data**.

## Format reference

The confirmed schemas with field-level notes live in the repository at `IMPORT_EXPORT_FORMATS.md`, and the implementations in `utils/importers/` (web) and `shellapp/internal/collection/` (terminal).
