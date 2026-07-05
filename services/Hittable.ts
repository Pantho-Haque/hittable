import { collections } from "@/constants";
import { THittableCollections } from "@/types";


export function GetHittableCollections() {
    if (typeof window === "undefined") return [];
    const storedCollections = localStorage.getItem("hittable");
    if (!storedCollections) return collections;
    try {
        return JSON.parse(storedCollections) as THittableCollections;
    } catch {
        return collections;
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