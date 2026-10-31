'use client';

import { ShortcutProvider } from "./ShortcutKeypressProvider";
import { DataProvider } from "./dataContext";

export default function ClientProviders({ children }: { children: React.ReactNode }) {
  return (
    <DataProvider>
      <ShortcutProvider>
        {children}
      </ShortcutProvider>
    </DataProvider>
  );
}
