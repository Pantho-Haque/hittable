# Import/Export Format Reference

## Supported Formats

### 1. Hittable Native (compressed string)
- **Export:** pako-compressed, URL-safe base64-encoded string
- **Import:** Auto-detected — input that fails JSON parsing or matches `THittableCollection` structure
- **Bidirectional:** Yes — full round-trip fidelity
- **Variable syntax:** `<<KEY>>` (native)

### 2. Postman Collection v2.1 (`.postman_collection.json`)
- **Schema:** https://schema.getpostman.com/json/collection/v2.1.0/collection.json
- **Import:** JSON with `info.schema` containing `schema.getpostman.com`
- **Export:** Valid v2.1 JSON importable into Postman

### 3. Insomnia Export (JSON with `_type` resource array)
- **Import:** JSON with `_type: "export"` or `__export_format: 4`, or array of resources with `_type` fields
- **Export:** JSON with `resources` array using Insomnia's `_type` convention

---

## Postman Collection v2.1 — Confirmed Schema

### Root Structure
```json
{
  "info": {
    "name": "Collection Name",
    "_postman_id": "...",
    "schema": "https://schema.getpostman.com/json/collection/v2.1.0/collection.json"
  },
  "item": [ /* items and/or item-groups */ ],
  "variable": [{ "key": "var", "value": "val", "type": "string" }],
  "auth": { "type": "bearer", "bearer": [{ "key": "token", "value": "..." }] }
}
```

### Items (requests)
```json
{
  "name": "Request Name",
  "request": {
    "method": "POST",
    "url": "https://api.example.com/users" | { "raw": "...", "host": [...], "path": [...] },
    "header": [{ "key": "Content-Type", "value": "application/json", "disabled": false }],
    "body": {
      "mode": "raw",
      "raw": "{\"key\": \"value\"}"
    }
  }
}
```

### Item Groups (folders) — recursive nesting
```json
{
  "name": "Folder Name",
  "item": [ /* nested items or item-groups */ ]
}
```

### Body Modes
| Mode | Content | Hittable Mapping |
|------|---------|------------------|
| `raw` | Freeform string (JSON, XML, etc.) | Direct to body field |
| `urlencoded` | `[{key, value, disabled}]` | JSON object body |
| `formdata` | `[{key, value/type/src, disabled}]` | File rows with `@filename` |
| `file` | `{src, content}` | Skipped (no file bytes in exports) |
| `graphql` | `{query, variables}` | Skipped (no GraphQL support) |

### Variable Syntax
- **Postman:** `{{varName}}` — translated to `<<varName>>` on import
- **Hittable:** `<<KEY>>` — translated to `{{KEY}}` on export

### Dropped on Import (with user notification)
- Pre-request scripts (`event.listen: "prerequest"`)
- Test scripts (`event.listen: "test"`)
- Auth types: OAuth1, OAuth2, AWS Signature v4, Digest, EdgeGrid, Hawk, NTLM
- Certificate configs
- Protocol profile behavior
- Proxy configs

### Dropped on Export
- Hittable notes
- Hittable history
- Hittable auth presets (only raw headers are exported)

---

## Insomnia Export — Confirmed Schema

### Root Structure
```json
{
  "_type": "export",
  "__export_format": 4,
  "__export_source": "insomnia.desktop",
  "resources": [ /* array of resources */ ]
}
```

### Resource Types
| `_type` | Description | Hittable Mapping |
|---------|-------------|------------------|
| `workspace` | Top-level container | Used for collection name |
| `request_group` | Folder (has `parentId`) | Collection name (if top-level) |
| `request` | HTTP request | Route |
| `environment` | Variable set | Env variables |
| `cookie_jar` | Cookie storage | Skipped |
| `api_spec` | OpenAPI spec | Skipped |

### Request Structure
```json
{
  "_type": "request",
  "_id": "req_xxx",
  "parentId": "fld_xxx",
  "name": "Get Users",
  "method": "GET",
  "url": "https://api.example.com/users",
  "headers": { "Authorization": "Bearer {{ _.token }}" },
  "body": {
    "mimeType": "application/json",
    "text": "{\"key\": \"value\"}"
  }
}
```

### Variable Syntax
- **Insomnia:** `{{ _.varName }}` (note the underscore prefix and spaces)
- **Hittable:** `<<KEY>>`

### Dropped on Import (with user notification)
- Plugin configurations
- Cookie jars
- Client certificates
- WebSocket/gRPC/socket.io requests
- Mock routes
- Unit tests
- API specs

---

## Flattening Strategy

### Postman → Hittable
- **Postman collections** map 1:1 to Hittable collections
- **Item groups (folders)** within a collection become Hittable collections if the collection only contains folders (no direct requests)
- If a collection has both direct requests and folders, the collection itself becomes a Hittable collection with the direct requests, and each folder becomes a separate collection
- **Nested folders** are flattened: folder path becomes prefix (e.g., "Users / Create" → route name "Users / Create")
- **Deep nesting** (3+ levels) is supported — all paths are joined with " / "

### Insomnia → Hittable
- **Top-level `request_group`** resources become Hittable collections
- If no request groups exist, all requests go into a single collection named after the workspace
- **Nested request groups** — only top-level groups are used as collection boundaries
- Requests inherit their `parentId` to determine collection membership

### Hittable → Postman
- Each Hittable collection becomes a Postman collection
- Each route (curl) becomes a Postman item
- Curl strings are parsed to extract method, URL, headers, and body
- Environment variables become collection-level variables

### Hittable → Insomnia
- Each Hittable collection becomes an Insomnia workspace + request_group
- Each route becomes an Insomnia request
- Environment variables become a base environment resource

---

## Variable Translation

| Source | Syntax | Target | Syntax |
|--------|--------|--------|--------|
| Postman | `{{varName}}` | Hittable | `<<varName>>` |
| Insomnia | `{{ _.varName }}` | Hittable | `<<varName>>` |
| Hittable | `<<KEY>>` | Postman | `{{KEY}}` |
| Hittable | `<<KEY>>` | Insomnia | `{{ _.KEY }}` |

---

## Edge Cases

1. **Postman URL as object** — Insomnia always uses raw string URLs. Postman can use either a raw string or a structured object (`{raw, host, path, query}`). The importer prefers `raw` and falls back to reconstructing from parts.

2. **Disabled headers/params** — Postman marks disabled items with `disabled: true`. These are skipped on import (not sent, not imported).

3. **File references in formdata** — Postman's `formdata` with `type: "file"` references files by path or filename. Since actual file bytes are never in exports, these become `@filename` references in Hittable's curl string. Users must re-attach files after import.

4. **Empty bodies** — GET/HEAD/DELETE requests with no body are handled correctly (no `-d` flag in curl generation).

5. **Collection-level vs request-level auth** — Postman supports both. On import, collection-level auth is noted but not directly mapped to Hittable (which has no auth persistence per collection). The auth headers are included in the request headers.

6. **Insomnia `kvPairData`** — Insomnia v5+ uses `_kvPairData` array instead of `data` object for environment variables. Both are supported on import.
