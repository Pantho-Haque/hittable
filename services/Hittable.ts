import { collections } from "@/constants";
import { THittableCollections } from "@/types";
import { migrateCollections } from "@/utils/treeHelpers";


export function GetHittableCollections(): THittableCollections {
    if (typeof window === "undefined") return [];
    const storedCollections = localStorage.getItem("hittable");
    if (!storedCollections) return collections as unknown as THittableCollections;
    try {
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
        const res = await fetch("https://panthohaque.vercel.app/api/resume", {
            method: "GET",
            headers: {
                "Content-Type": "application/json",
            },
            cache: "no-store",
        });
        return res.json();
    } catch {
        return { hero: { name: "Pantho Haque", current_position: "Software Engineer I", company_name: "Pathao Ltd.", comment_one: "", comment_two: "", contactLinks: [] }, experience: [{ stack: [] }] };
    }
}
