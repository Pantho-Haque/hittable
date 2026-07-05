
import pako from "pako";

export const compressString = (raw: string): string => {
  const compressed = pako.deflate(new TextEncoder().encode(raw));
  let binary = "";
  for (let i = 0; i < compressed.length; i++) {
    binary += String.fromCharCode(compressed[i]);
  }
  return btoa(binary)
    .replace(/\+/g, "-")
    .replace(/\//g, "_")
    .replace(/=+$/, "");
};

export const decompressString = (code: string): string => {
  try {
    const base64 = code.replace(/-/g, "+").replace(/_/g, "/");
    const binary = atob(base64);
    const bytes = Uint8Array.from(binary, (c) => c.charCodeAt(0));
    return new TextDecoder().decode(pako.inflate(bytes));
  } catch {
    throw new Error("Invalid or corrupted import data");
  }
};
