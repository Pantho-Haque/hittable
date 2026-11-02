// Bridges the Hittable page and the background worker. Only messages the
// page posts to itself are relayed; replies carry the request id back.
window.addEventListener("message", (event) => {
  if (event.source !== window || event.origin !== window.location.origin) return;

  if (event.data?.type === "HITTABLE_PING") {
    // "binary": file uploads (a base64 body), since 1.1.0.
    window.postMessage({ type: "HITTABLE_EXTENSION_READY", capabilities: ["binary"] }, window.location.origin);
  }

  if (event.data?.type === "HITTABLE_REQUEST") {
    const id = event.data.id;
    chrome.runtime.sendMessage(event.data, (response) => {
      const reply = response ?? { success: false, error: chrome.runtime.lastError?.message || "Extension did not answer" };
      window.postMessage({ type: "HITTABLE_RESPONSE", id, ...reply }, window.location.origin);
    });
  }
});
