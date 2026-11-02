import type { THittableCurlJson, THittableEnv } from "@/types";

/**
 * The request with every <<KEY>> that env defines replaced by its value.
 * Unknown keys stay as written. Values are inserted literally (no $-patterns,
 * no re-expansion), and only own keys count, so "<<constructor>>" stays put.
 */
export function resolveEnv(formInput: THittableCurlJson, env?: THittableEnv): THittableCurlJson {
  if (!env) return formInput;
  const out: THittableCurlJson = { ...formInput };
  (Object.entries(out) as [string, unknown][]).forEach(([key, val]) => {
    if (typeof val === "string") {
      (out as Record<string, unknown>)[key] = val.replace(/<<(\w+)>>/g, (match, envKey: string) =>
        Object.hasOwn(env, envKey) ? env[envKey] : match,
      );
    }
  });
  return out;
}
