import { NotificationProvider } from "./notifyContext";
import ClientProviders from "./ClientProviders";
import { WorkspaceProvider } from "./workspaceContext";

export default function Providers({ children }: { children: React.ReactNode }) {
  return (
    <ClientProviders>
      <WorkspaceProvider>
        <NotificationProvider>{children}</NotificationProvider>
      </WorkspaceProvider>
    </ClientProviders>
  );
}
