import type { THittableSelectorSelection } from "@/types";

/** The /hittable URL that selects a collection, folder and route (the Selector reads it back). */
export function selectionHref({ collectionName: c, curlName: r, folderPath: p }: Partial<THittableSelectorSelection>): string {
  if (!c) return "/hittable";
  const pathParam = p && p.length > 0 ? `&p=${encodeURIComponent(p.join("/"))}` : "";
  const routeParam = r ? `&r=${encodeURIComponent(r)}` : "";
  return `/hittable?c=${encodeURIComponent(c)}${routeParam}${pathParam}`;
}
