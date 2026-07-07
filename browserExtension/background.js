// background.js
chrome.runtime.onMessage.addListener((message, _, sendResponse) => {
  if (message.type !== "HITTABLE_REQUEST") return;

  const { url, method, headers, body } = message.payload;
  const upperMethod = method.toUpperCase();

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
