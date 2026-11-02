import { ShortcutProvider } from "@/context/ShortcutKeypressProvider";
import { WorkspaceProvider } from "@/context/workspaceContext";
import CommandPalette from "@/components/palette/CommandPalette";
import ShortcutsModal from "@/components/modals/ShortcutsModal";
import SharePreview from "@/components/workspace/SharePreview";

// The workspace stays open while moving between the launcher and a workspace.
export default function HittableLayout({ children }: { children: React.ReactNode }) {
  return (
    <ShortcutProvider>
      <WorkspaceProvider>
        {children}
        <CommandPalette />
        <ShortcutsModal />
        <SharePreview />
      </WorkspaceProvider>
    </ShortcutProvider>
  );
}
