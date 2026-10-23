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

// Same names the terminal app accepts (shellapp/internal/collection).
const POSTMAN_VAR = /\{\{\s*([A-Za-z0-9_.-]+)\s*\}\}/g;
const INSOMNIA_VAR = /\{\{\s*_\.([A-Za-z0-9_.-]+)\s*\}\}/g;
const HITTABLE_VAR = /<<(\w+)>>/g;

export const fromPostman = (s: string) => s.replace(POSTMAN_VAR, "<<$1>>");
// `{{ _.KEY }}` first, so the generic `{{KEY}}` pass never sees the `_.` prefix.
export const fromInsomnia = (s: string) => fromPostman(s.replace(INSOMNIA_VAR, "<<$1>>"));
export const toPostman = (s: string) => s.replace(HITTABLE_VAR, "{{$1}}");
export const toInsomnia = (s: string) => s.replace(HITTABLE_VAR, "{{ _.$1 }}");

// Adds an empty env entry for every <<KEY>> used by a request, keeping values.
export function addMissingVars(text: string, env: Record<string, string>): void {
  for (const m of text.matchAll(HITTABLE_VAR)) if (!(m[1] in env)) env[m[1]] = "";
}
