/**
 * Extracts variable names from a string containing Postman/Insomnia-style
 * template variables and returns them as a set.
 */
export function extractVariableNames(text: string): Set<string> {
  const names = new Set<string>();
  // Postman: {{varName}}
  for (const m of text.matchAll(/\{\{(\w+)\}\}/g)) {
    names.add(m[1]);
  }
  // Insomnia: {{ _.varName }}
  for (const m of text.matchAll(/\{\{\s*_\.(\w+)\s*\}\}/g)) {
    names.add(m[1]);
  }
  return names;
}

/**
 * Ensures env var entries exist for every variable reference found in `text`.
 * Only adds missing keys (empty string value) — never overwrites existing values.
 */
export function ensureEnvVarsForText(
  text: string,
  env: Record<string, string>,
): Record<string, string> {
  const updated = { ...env };
  for (const name of extractVariableNames(text)) {
    if (!(name in updated)) {
      updated[name] = "";
    }
  }
  return updated;
}
