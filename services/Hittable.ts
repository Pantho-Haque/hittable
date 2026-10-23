import { collections } from "@/constants";
import { THittableCollections } from "@/types";
import { migrateCollections } from "@/utils/treeHelpers";


export function GetHittableCollections(): THittableCollections {
    if (typeof window === "undefined") return [];
    try {
        // Inside the try: with storage blocked, merely reading localStorage throws.
        const storedCollections = localStorage.getItem("hittable");
        if (!storedCollections) return collections as unknown as THittableCollections;
        const parsed = JSON.parse(storedCollections);
        const migrated = migrateCollections(parsed);
        // Persist migrated data back so migration only runs once
        if (JSON.stringify(migrated) !== JSON.stringify(parsed)) {
            try {
                localStorage.setItem("hittable", JSON.stringify(migrated));
            } catch { /* quota exceeded */ }
        }
        return migrated;
    } catch {
        return collections as unknown as THittableCollections;
    }
}

export async function GetResume() {
    try {
        // The home page waits on this, so a slow or down portfolio API must not
        // hang it: give up after 3s and use the fallback; cache good answers.
        const res = await fetch("https://panthohaque.vercel.app/api/resume", {
            headers: { Accept: "application/json" },
            signal: AbortSignal.timeout(3000),
            next: { revalidate: 3600 },
        });
        if (!res.ok) throw new Error(`resume API ${res.status}`);
        return await res.json();
    } catch {
        return { hero: { name: "Pantho Haque", current_position: "Software Engineer I", company_name: "Pathao Ltd.", comment_one: "", comment_two: "", contactLinks: [] }, experience: [{ stack: [] }] };
    }
}
