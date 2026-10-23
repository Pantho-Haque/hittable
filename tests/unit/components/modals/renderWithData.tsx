import { render } from "@testing-library/react";
import { useEffect, type ReactNode } from "react";
import { DataProvider, useDataContext } from "@/context/dataContext";
import { NotificationProvider } from "@/context/notifyContext";
import type { THittableCollections, THittableCurlJson, THittableSelectorResponse } from "@/types";

// Renders a component inside the real app providers, seeded through
// localStorage the same way the app boots.
export function renderWithData(
  ui: ReactNode,
  {
    collections = [],
    selector,
    formInput,
  }: {
    collections?: THittableCollections;
    selector?: THittableSelectorResponse | null;
    formInput?: Partial<THittableCurlJson>;
  } = {},
) {
  localStorage.setItem("hittable", JSON.stringify(collections));
  const ctxRef: { current: ReturnType<typeof useDataContext> | null } = { current: null };
  function Probe() {
    const c = useDataContext();
    useEffect(() => {
      ctxRef.current = c;
    });
    return null;
  }
  function Seed({ children }: { children: ReactNode }) {
    const { setSelectorResponse, setFormInput } = useDataContext();
    // Seed once on mount, like a user who already opened a route.
    useEffect(() => {
      if (selector !== undefined) setSelectorResponse(selector);
      if (formInput) setFormInput((prev) => ({ ...prev, ...formInput }));
      // eslint-disable-next-line react-hooks/exhaustive-deps
    }, []);
    return <>{children}</>;
  }
  const result = render(
    <NotificationProvider>
      <DataProvider>
        <Seed>
          <Probe />
          {ui}
        </Seed>
      </DataProvider>
    </NotificationProvider>,
  );
  return { ...result, ctx: () => ctxRef.current! };
}

/** Collections as persisted by the app (what survives a reload). */
export const stored = (): THittableCollections => JSON.parse(localStorage.getItem("hittable") ?? "[]");
