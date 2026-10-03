---
title: Project format
description: The on-disk format both apps share. A hittable/ folder, .hit request files, env.json for variables, markdown notes. Plain JSON, git-friendly, no database.
---

Hittable has no account and no hidden store. A project is a folder in your repository, and everything the web app and the terminal app know about your API lives in files you can read, diff and commit.

## Folder layout

`hittable init` in the terminal, or choosing a folder in the web app's Directory mode, scaffolds this:

```
<your repo>/
└── hittable/
    ├── env.json                 variables shared by every request in the folder
    ├── testcollection/
    │   └── test.hit             a real, runnable request
    └── notes/
        └── sample.md            markdown notes, rendered in both apps
```

Any folder under `hittable/` is a collection; collections nest. A request is a `.hit` file. Both apps read the folder as-is, in on-disk order, and never create files unless you ask.

## The `.hit` file

```json
{
  "method": "GET",
  "url": "<<BASE_URL>>/users/3",
  "headers": { "Authorization": "Bearer <<AUTH_TOKEN>>" },
  "params": {},
  "body": "",
  "response": null
}
```

| Field | Type | Rules |
| :--- | :--- | :--- |
| `method` | string | `GET`, `POST`, `PUT`, `PATCH`, `DELETE`, `HEAD`, `OPTIONS` |
| `url` | string | may contain `<<KEY>>` templates; a URL without `://` gets `http://` |
| `headers` | object | flat string → string, never an array |
| `params` | object | flat string → string; appended to the URL, URL-encoded, at send time |
| `body` | string | raw text, which may itself be escaped JSON, or empty |
| `response` | object or `null` | `null` until the request has been sent at least once from this file |

After a send, `response` holds the last result so it travels with the request:

```json
"response": {
  "data": { "id": 3, "name": "Clementine Bauch" },
  "status": 200,
  "statusText": "OK",
  "ok": true,
  "headers": { "content-type": "application/json; charset=utf-8" },
  "cookies": {},
  "durationMs": 182,
  "sizeBytes": 509
}
```

Files are written with two-space indentation and without HTML escaping, so `<<KEY>>` stays literal and diffs stay readable.

## `env.json` and templates

`hittable/env.json` is a flat map, shared by every `.hit` file anywhere under `hittable/`, however deeply nested:

```json
{
  "BASE_URL": "https://api.example.com",
  "AUTH_TOKEN": ""
}
```

`<<KEY>>` tokens in the URL, in header values, in param values and in the body are resolved against it **at send time only**. The template stays in the file. Neither app ever shows you, or writes back, an interpolated value, which is what makes the files safe to commit. An unknown key is left as the literal `<<KEY>>` so you notice it in the response.

Postman's `{{var}}` and Insomnia's `{{ _.var }}` become `<<var>>` on import and are translated back on export.

## Notes

Any `.md` file in the folder is a note. The terminal app renders it with a live preview (including Mermaid diagrams); the web app renders it with the same markdown flavour in its editor. Keep API docs next to the requests that exercise them.

## Sharing between the two apps

The terminal app and the web app open the same folder with no conversion step. Edit a request in the browser, run it from the terminal; stage and commit from the terminal's Git panel; your teammates `git pull` and open the folder in whichever tool they prefer. Autosave in both apps writes on a short debounce and both detect files changed underneath them, so switching tools mid-session is fine.

A live example lives in this repository at `hittable/` (`env.json`, `testcollection/test.hit`, `notes/sample.md`).

## Import and export

Postman v2.1 and Insomnia v4 collections convert both ways. See [Import and export](/docs/import-export) for the mapping and what gets dropped.
