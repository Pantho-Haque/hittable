---
title: Import and export
description: Move collections between Hittable, Postman and Insomnia with the terminal app. What maps, how variables translate, and what has no equivalent and is dropped.
---

Postman and Insomnia conversion lives in the terminal app. The web app has no importer; import once from the terminal, commit the `hittable/` folder, and the web app opens the result like any other `.hit` files.

| Format | Detected by | Export file |
| :--- | :--- | :--- |
| Postman Collection v2.1 | `info.schema` containing `getpostman.com` | `<folder>.postman_collection.json` |
| Insomnia export v4 | `_type: "export"` with a `resources` array, or a bare array of `_type` resources | `<folder>.insomnia.json` |

## Commands

```sh
hittable -i collection.json          # import a Postman or Insomnia export, then open
hittable -e postman                  # export hittable/ as <folder>.postman_collection.json
hittable -e insomnia                 # export hittable/ as <folder>.insomnia.json
hittable -e postman -o api.json      # pick the file name
```

Import puts the collection under `hittable/<Collection name>/`, folders as directories, requests as `.hit` files, and merges variables into `hittable/env.json` without overwriting keys that already exist. It prints a report: requests, folders, variables, and everything that was dropped. Export walks `hittable/` recursively. The round trip is tested. The full flag list is in the [CLI reference](/docs/shellapp-cli).

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
| `item` with `request` | a request (`.hit` file) |
| `item` with nested `item` | a folder |
| `url` as a string or as an object with `host`, `path`, `query` | `url` plus `params` from the query |
| `header[]` | `headers`, disabled rows skipped |
| body `raw` | `body` as-is |
| body `urlencoded` | a JSON object body |
| body `formdata` | key / value rows; file entries become `@filename` |
| body `file`, `graphql` | skipped |
| `auth` bearer, basic, apikey | the equivalent `Authorization` or custom header, with inheritance from folder and collection and `noauth` honoured |
| items marked `disabled: true` | skipped |

Dropped on import, and reported: pre-request and test scripts; OAuth1, OAuth2, AWS Signature v4, Digest, EdgeGrid, Hawk and NTLM auth; certificate, protocol-profile and proxy configuration. Saved responses and notes are not exported.

## Insomnia mapping

| Insomnia resource | Hittable |
| :--- | :--- |
| `workspace` | the collection |
| `request_group` | a folder, nested by `parentId` |
| `request` | a request; `parameters` become `params`, `headers` become `headers`, `body.text` or `body.params` become `body` |
| `environment` | variables, merged |

Dropped on import, and reported: plugin configuration, cookie jars, client certificates, WebSocket, gRPC and socket.io requests, mock routes, unit tests and API specs.

## Format reference

The confirmed schemas with field-level notes live in the repository at `IMPORT_EXPORT_FORMATS.md`, and the implementation in `shellapp/internal/collection/`.
