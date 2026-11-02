import { startLogin } from "@/utils/server/github/oauth";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

export function GET(req: Request) {
  return startLogin(req);
}
