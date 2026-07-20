# UX Plan — Response View Modes (Text / JSON / HTML)

## How the three response view modes fit the user's workflow

### When users reach for each mode

- **JSON mode** (default for `application/json`): The primary view for API development. Users inspect nested structures, expand/collapse objects, and search for specific keys or values. This is where 80%+ of API work happens — checking response payloads, verifying field presence, validating data shapes.

- **Text mode** (default for unknown content-types): The fallback and the "raw truth" view. Users reach for this when JSON parsing fails (e.g. a server returns malformed JSON with a `application/json` header), when they need to see exact whitespace/formatting, or when the response is plain text, CSV, XML, or any non-JSON format. Also useful for debugging — seeing the exact bytes the server sent.

- **HTML mode** (default for `text/html`): For testing endpoints that return HTML fragments, email templates, webhook payloads, or error pages. Also valuable when an API unexpectedly returns HTML (e.g. a redirect page, a login form, a rate-limit page). The sandboxed preview lets users see what the response would look like rendered, while the source view below lets them inspect the actual markup.

### Auto-detection reduces friction meaningfully

Content-Type based auto-detection eliminates the "which view do I need?" decision for the common cases. When a JSON API returns JSON, users land directly in the tree view without clicking. When a webhook sends HTML, they see the rendered preview immediately. The manual override (clicking any mode icon) handles the 20% of cases where auto-detection picks wrong or the user wants a different perspective on the same data.

The auto-detection resets on each new response but respects manual overrides within a session — so if a user manually switches to Text mode to inspect raw bytes, they stay in Text mode for subsequent responses until they send a new request. This prevents the annoying "it keeps switching back" behavior while still being smart by default.

## Unifying Body (request) and Response Panel view modes

### What's unified

- **Same three modes**: Text, JSON, HTML — identical mental model whether editing a request body or viewing a response
- **Same icon set**: Braces (JSON), FileText (Text), Globe (HTML) — consistent visual language
- **Same interaction pattern**: Click icon to switch, active state shown with cyan highlight + ring
- **Same search behavior**: Cmd/Ctrl+F opens floating search, works on raw text content in all modes
- **Same copy behavior**: Copy available in Text and JSON modes, hidden in HTML mode (per the established rule)
- **Same sandboxing approach**: HTML preview uses `iframe` with `sandbox="allow-same-origin"` in both contexts

### Intentional divergences

- **Request Body has Table mode** (for key-value editing of form data) — Response Panel doesn't need this since responses are read-only
- **Request Body has line numbers** (for editing) — Response Panel JSON view uses the tree renderer (read-only), so line numbers aren't applicable. Text mode could theoretically have them, but raw response text is typically viewed, not edited, so they'd add visual noise without value
- **Request Body has Beautify button** (Cmd/Ctrl+J for formatting) — Response Panel doesn't need this since it doesn't edit content

These divergences are justified: the request editor is a write surface, the response panel is a read surface. The shared elements (mode icons, search, copy, HTML sandboxing) create consistency where it matters for the user's mental model.

## Opportunities to make working with responses feel smoother

### 1. Response content-type badge in the header bar

When a response arrives, show a small badge next to the status/duration/size metrics indicating the detected content-type (e.g. "JSON", "HTML", "Text"). This gives users immediate context about what kind of response they got without having to open the Headers tab. Especially useful when debugging APIs that return unexpected content types.

### 2. "Open HTML in new tab" action in HTML mode

When viewing an HTML response, add a small icon button in the HTML preview header that opens the response in a new browser tab (using a blob URL). This lets users interact with the HTML normally — click links, inspect with DevTools, test forms — without leaving Hittable. The sandboxed iframe is great for safety but limits interactivity; a "pop out" option bridges that gap.

### 3. Response size warning for very large payloads

When a response exceeds a threshold (e.g. 1MB), show a subtle warning in the response panel header suggesting the user switch to Text mode or use search to find specific content, rather than rendering a massive JSON tree or HTML preview that could slow down the UI. This could be a small amber badge: "Large response (2.4 MB) — consider Text mode for better performance".
