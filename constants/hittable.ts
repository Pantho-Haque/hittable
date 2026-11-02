
export const HITTABLE_METHODS = [
  "GET",
  "POST",
  "PUT",
  "PATCH",
  "DELETE",
  "HEAD",
  "OPTIONS",
];


// Same values as the --method-* tokens in styles/theme.css. Kept as hex
// because callers append an alpha suffix (`${color}33`).
export const METHOD_COLORS: Record<string, string> = {
  GET: "#4ade80",
  POST: "#fbbf24",
  PUT: "#60a5fa",
  PATCH: "#c084fc",
  DELETE: "#f87171",
  HEAD: "#a0a8b4",
  OPTIONS: "#a0a8b4",
};
