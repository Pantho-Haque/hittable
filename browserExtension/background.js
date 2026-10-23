// background.js
chrome.runtime.onMessage.addListener((message, _, sendResponse) => {
  if (message.type !== "HITTABLE_REQUEST") return;

  const { url, method, headers, body } = message.payload ?? {};
  // The extension exists for localhost only; refuse anything else.
  let host;
  try {
    host = new URL(url).hostname;
  } catch {
    sendResponse({ success: false, error: `Invalid URL: ${url}` });
    return;
  }
  if (!["localhost", "127.0.0.1", "0.0.0.0"].includes(host)) {
    sendResponse({ success: false, error: "The extension only makes localhost requests" });
    return;
  }
  const upperMethod = String(method || "GET").toUpperCase();

  fetch(url, {
    method: upperMethod,
    headers: headers || {},
    body: ["GET", "HEAD"].includes(upperMethod) ? undefined : body ?? null,
  })
    .then(async (res) => {
      // ─── Data ────────────────────────────────────────────────────────────
      const contentType = res.headers.get("content-type") ?? "";
      const data = contentType.includes("application/json")
        ? await res.json()
        : await res.text();

      // ─── Headers ─────────────────────────────────────────────────────────
      const responseHeaders = {};
      res.headers.forEach((value, key) => {
        responseHeaders[key] = value;
      });

      // ─── Cookies ─────────────────────────────────────────────────────────
      const cookieUrl = new URL(url).origin;
      chrome.cookies.getAll({ url: cookieUrl }, (cookies) => {
        sendResponse({
          success: true,
          data,
          status: res.status,
          statusText: res.statusText,
          ok: res.ok,
          headers: responseHeaders,
          cookies: cookies ?? [],
        });
      });
    })
    .catch((err) => {
      sendResponse({
        success: false,
        error: err.message,
      });
    });

  return true; // ← keeps sendResponse alive for async
});
