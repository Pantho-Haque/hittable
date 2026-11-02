import { ApiError, ghFetch } from "@/utils/server/github/client";
import { githubRoute, isSha, repoPath } from "@/utils/server/github/handler";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

// 4 MB: under Vercel's 4.5 MB cap on a buffered function response, so a file
// either loads or gets a clear "too large", never a platform error.
const MAX_BLOB_BYTES = 4 * 1024 * 1024;

const tooLarge = () => new ApiError(413, "too_large", `Files over ${MAX_BLOB_BYTES} bytes are not loaded`);

// Raw bytes by SHA. A SHA names its content, so the answer is cacheable forever.
export const GET = githubRoute<{ owner: string; repo: string; sha: string }>(async ({ gh, params }) => {
  if (!isSha(params.sha)) throw new ApiError(400, "invalid", "Invalid blob SHA");
  const res = await ghFetch(gh, `${repoPath(params)}/git/blobs/${params.sha}`, { accept: "application/vnd.github.raw+json" });
  if (Number(res.headers.get("content-length")) > MAX_BLOB_BYTES) {
    await res.body?.cancel();
    throw tooLarge();
  }
  const chunks: Uint8Array[] = [];
  let size = 0;
  const reader = res.body!.getReader();
  for (;;) {
    const { done, value } = await reader.read();
    if (done) break;
    size += value.byteLength;
    if (size > MAX_BLOB_BYTES) {
      await reader.cancel();
      throw tooLarge();
    }
    chunks.push(value);
  }
  return new Response(Buffer.concat(chunks), {
    headers: {
      "Content-Type": "application/octet-stream",
      "Cache-Control": "private, max-age=31536000, immutable",
    },
  });
});
