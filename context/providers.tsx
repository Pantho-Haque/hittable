"use client";

import { NotificationProvider } from "./notifyContext";

// App-wide only. The workspace providers live in app/hittable/layout.tsx.
export default function Providers({ children }: { children: React.ReactNode }) {
  return <NotificationProvider>{children}</NotificationProvider>;
}
