# File uploads in `.hit` files: the shared contract

The web app and the terminal app both implement this. The format adds **no new keys**: older versions of either app keep the file intact (the terminal app drops unknown keys on save).

Parsing is pinned by one table of cases, kept in two identical copies:
- `tests/unit/fixtures/fileBody.cases.json` (web)
- `shellapp/internal/hitfile/testdata/filebody_cases.json` (terminal)

A web unit test fails if the two copies differ.

## Parse: `parseFileRefs(body, contentType)`

`contentType` is the value of the request's `Content-Type` header (case-insensitive key), or `""`.

1. **Multipart.** `contentType`, lower-cased, starts with `multipart/form-data`.
   - `body` trimmed empty: no parts.
   - Otherwise `body` must be a JSON object, or it is an error. Its members become parts in the order JavaScript's `Object.entries(JSON.parse(body))` gives:
     - document order, except that keys which are array indices (`"0"`, `"10"`) come first, in numeric order;
     - a repeated key keeps its first position and its last value.

     The terminal app reproduces this exactly.
   - String value `"@@…"`: a text part with one `@` removed.
   - String value `"@…"`: a file part. The rest is a file spec, and an empty path is an error.
   - Any other string: a text part.
   - Non-string values: `null` becomes `""`; everything else becomes its compact JSON text (`42`, `true`, `{"a":1}`).
     - The web app re-serializes the value, so `1.0` is sent as `1` and `"\u00e9"` as `é`.
     - The terminal app sends the text as written.
     - Write such values as strings to get the same bytes from both.
2. **Binary.** Otherwise, take `t = body.trim()`. If `t` starts with `@`, has no `\n` and is longer than 1 character:
   - `t` starts with `@@`: `raw`, and the body sent is `t` with one `@` removed.
   - Otherwise: a binary file body, with file spec `t[1:]`. An empty path is an error.

   This applies whatever the content type is (a JSON file can be the body).
3. Otherwise: no file references (`null`). The body is sent as it is.

**File spec:** `path[;type=T][;filename=F]`.
- Only trailing `;type=` and `;filename=` segments, in any order, are options. Any other `;` stays part of the path.
- The path is trimmed. Option values are taken as written.

**Result shapes** (as in the cases file):
- `null`
- `{kind:"raw", body}`
- `{kind:"binary", file}`
- `{kind:"multipart", parts:[{name, value} | {name, file}]}`

where `file` is `{path, type?, filename?}`. An error is any failure; the cases file only says `{"error": true}`.

`<<KEY>>` variables are resolved in the body **before** parsing when sending, so they work inside paths.

## Resolve paths

Paths are relative to **the folder of the `.hit` file**, and `/` is the separator (`\` is read as `/`).

- **Web:** absolute paths (`/x`, `~/x`, `C:\x`) are an error ("absolute paths only work in the terminal app"). A path whose `..` climbs above the workspace root is an error.
- **Terminal:** relative paths resolve against the `.hit` file's folder. Absolute paths and `~/` are allowed.

## Send

**Multipart**
- Built with the platform's multipart writer.
- The `Content-Type` is replaced by `multipart/form-data; boundary=<generated>`.
- File part filename: the `filename` option, or else the path's base name.
- File part content type: the `type` option, or else a guess from the extension, or else `application/octet-stream`.

**Binary**
- The file's bytes are the body.
- `Content-Type`: the request's own header if set, or else the `type` option, or else a guess from the extension, or else `application/octet-stream`.

**Errors**
- A missing or unreadable file: nothing is sent, and the error names the path.
- **Web limits:** the proxy accepts at most `PROXY_MAX_REQUEST_BYTES` (4 MiB by default; Vercel caps request bodies at 4.5 MB). GitHub repositories serve files up to 4 MB, and Git LFS files cannot be read from the web.

## Where uploads live

Files picked from the computer in the web app are copied to `hittable/files/<name>`, with `-1`, `-2`… before the extension on a clash. The body stores the path relative to the `.hit` file. Nothing requires that folder: any file in the workspace (or anywhere, in the terminal) can be referenced.

## curl

- Multipart: `-F 'name=value'`, `-F 'f=@path;type=T;filename=F'`.
- Binary: `--data-binary @path`, plus `-H 'Content-Type: …'` with the type the app would send (curl would otherwise send `application/x-www-form-urlencoded`).
- Copied paths stay as written, relative to the `.hit` file, so run the command from that folder.
- Import also reads `-T path` / `--upload-file path` as a binary body.
