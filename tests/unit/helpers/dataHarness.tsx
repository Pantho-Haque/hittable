// Mounts the real Local-mode providers with one route selected, the way the
// Selector does it, and exposes the live data context for assertions.
import { useEffect, type ReactNode } from "react";
import { render } from "@testing-library/react";
import { DataProvider, useDataContext } from "@/context/dataContext";
import { NotificationProvider } from "@/context/notifyContext";
import { curlConverter } from "@/utils/curlConverter";
import { findRoute } from "@/utils/treeHelpers";
import type { THittableCollections } from "@/types";

export type DataCtx = ReturnType<typeof useDataContext>;

export const sampleCollections = (): THittableCollections => [
  {
    collectionName: "Users",
    items: [
      { type: "route", name: "List users", curl: `curl -X GET https://<<HOST>>/users -H "Authorization: Bearer <<TOKEN>>"`, response: "" },
      {
        type: "folder",
        name: "Admin",
        items: [{ type: "route", name: "Create user", curl: `curl -X POST https://<<HOST>>/admin/users -d '{"name":"Ada"}'`, response: "" }],
      },
    ],
    env: { HOST: "api.example.com", TOKEN: "env-token" },
    secrets: { TOKEN: "secret-token" },
  },
  { collectionName: "Other", items: [], env: {}, secrets: {} },
];

export function mountData(
  ui: ReactNode,
  { collections = sampleCollections(), collection = "Users", folderPath = [] as string[], route = "List users" } = {},
) {
  localStorage.setItem("hittable", JSON.stringify(collections));
  const ctxRef: { current: DataCtx | null } = { current: null };
  function Select() {
    const ctx = useDataContext();
    useEffect(() => { ctxRef.current = ctx; });
    const { setSelectorResponse } = ctx;
    useEffect(() => {
      const col = collections.find((c) => c.collectionName === collection)!;
      const r = findRoute(col.items, folderPath, route);
      setSelectorResponse({
        collectionName: collection,
        folderPath,
        curlName: route,
        env: col.env,
        secrets: col.secrets,
        curlJson: curlConverter(r?.curl ?? ""),
        responseJson: r?.response ? JSON.parse(r.response) : null,
      });
    }, [setSelectorResponse]);
    return null;
  }
  const utils = render(
    <NotificationProvider>
      <DataProvider>
        <Select />
        {ui}
      </DataProvider>
    </NotificationProvider>,
  );
  return { ...utils, ctx: () => ctxRef.current! };
}
