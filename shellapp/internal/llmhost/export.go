package llmhost

import (
	"context"
	"net/http"
)

// DownloadVerified fetches url into ~/.hittable/tmp/<file>.part with the same
// resume, retry and checksum rules as the model install, and returns the path
// of the verified file. size 0 skips the size check; sum is the hex SHA-256.
// Other installers (the Node.js runtime, VS Code extensions) use it so there is
// one download path to get right.
func DownloadVerified(ctx context.Context, label, url, file string, size int64, sum string, p Progress) (string, error) {
	in := &installer{client: &http.Client{}}
	return in.fetch(ctx, artifact{label: label, url: url, file: file, bytes: size, sum: sum}, p)
}

// ExtractArchive unpacks a "zip" or "tar.gz" archive into dest, refusing any
// entry that would land outside it. File modes from the archive are kept.
func ExtractArchive(kind, src, dest string) error { return extractArchive(kind, src, dest) }

// SHA256File is the hex SHA-256 of a file.
func SHA256File(path string) (string, error) { return sha256File(path) }

// FreeDisk is the free space in bytes on the volume holding dir (or its
// nearest existing parent), 0 when it cannot be read.
func FreeDisk(dir string) int64 { return freeDisk(dir) }
