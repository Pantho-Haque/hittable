package llmhost

import (
	"fmt"
	"path"
	"runtime"
)

// Everything that identifies *what* gets downloaded lives in this one block.
// Moving to a newer llama.cpp or a different model is an edit here and nowhere
// else. Every size and hash below was read from the live release, not
// estimated: a wrong hash is indistinguishable from a corrupt download, and the
// error it produces would be a lie.
const (
	// RuntimeTag is the pinned llama.cpp release.
	RuntimeTag = "b11120"

	// ModelFileName is the GGUF as it is named on disk and on the hub.
	ModelFileName = "qwen2.5-coder-3b-instruct-q4_k_m.gguf"
	// ModelLabel is what the user is shown. ModelFileName is what is written.
	ModelLabel = "qwen2.5-coder 3B instruct · Q4_K_M"
	// ModelDownloadBytes is the exact size of the GGUF: 2.10 GB decimal.
	ModelDownloadBytes = 2104932800
	ModelSHA256        = "724fb256bec1ff062b2f65e4569e871ad2e95ab2a3989723d1769c54294730b7"

	// ModelRAMBytes is roughly what the server is resident at while generating:
	// the Q4_K_M weights plus an 8k KV cache and the compute buffers.
	ModelRAMBytes = 3_200_000_000

	// ContextSize is the ceiling every prompt budget is sized against. Leaving
	// it at the server default is the classic cause of a silently truncated
	// prompt, so it is set explicitly and named here rather than at the call.
	ContextSize = 8192
)

// Origins are separate from the file names so a test can point the whole
// installer at an httptest server without touching the asset table.
const (
	runtimeOrigin = "https://github.com/ggml-org/llama.cpp/releases/download/" + RuntimeTag + "/"

	// Pinned to an immutable commit rather than /main: /main moves, and the
	// hash above would go stale without anything reporting that it had.
	modelOrigin = "https://huggingface.co/Qwen/Qwen2.5-Coder-3B-Instruct-GGUF/resolve/" +
		"f74adce6aa16316c625447af059dbebe4983757c/"
)

// asset is one prebuilt runtime archive.
type asset struct {
	File  string // name under runtimeOrigin
	Label string // the platform, as the release names it
	Bytes int64
	SHA   string
}

// runtimeAssets is keyed by GOOS/GOARCH, read at run time. That is the whole
// reason this package has no build tags: an unsupported platform is a missing
// map entry and a readable message, not a compile error.
//
// macOS and Linux ship .tar.gz; only Windows ships .zip. Both are handled.
var runtimeAssets = map[string]asset{
	"darwin/arm64": {
		File: "llama-b11120-bin-macos-arm64.tar.gz", Label: "macos-arm64",
		Bytes: 11205150, SHA: "f34a2df471a1a6cb4fafd5dacd5f63a54fdc43971235df4d05634ec760eff106",
	},
	"darwin/amd64": {
		File: "llama-b11120-bin-macos-x64.tar.gz", Label: "macos-x64",
		Bytes: 11235287, SHA: "538689fdbb2695a416f4c57baaaa4cf0368774ea22f7180c2ef0e5e1f3eccdfd",
	},
	"linux/amd64": {
		File: "llama-b11120-bin-ubuntu-x64.tar.gz", Label: "ubuntu-x64",
		Bytes: 16997245, SHA: "cae70da61dc74e3012418f7a4b6750eadc6fd0a8fb03270022d471b675e5b4cd",
	},
	"linux/arm64": {
		File: "llama-b11120-bin-ubuntu-arm64.tar.gz", Label: "ubuntu-arm64",
		Bytes: 13588955, SHA: "32a3b84113d3439dfde3ba8488b644ff18e5a2847a8556f18e5e7ee65acf30dc",
	},
	"windows/amd64": {
		File: "llama-b11120-bin-win-cpu-x64.zip", Label: "win-cpu-x64",
		Bytes: 18558140, SHA: "bbaf0584954c7ef53c2ccc5675f96b8849ac81ca181f540b862eefef1a09a402",
	},
	"windows/arm64": {
		File: "llama-b11120-bin-win-cpu-arm64.zip", Label: "win-cpu-arm64",
		Bytes: 12033016, SHA: "bde095ca8437d3b042781910baf79bcd813146bc87c1a7429ec95daaa0da86f0",
	},
}

// platformKey is the map key for the machine this process is running on.
func platformKey(goos, goarch string) string { return goos + "/" + goarch }

// runtimeAsset looks up the archive for a platform. The bool is false for any
// platform llama.cpp does not publish a CPU build for.
func runtimeAsset(goos, goarch string) (asset, bool) {
	a, ok := runtimeAssets[platformKey(goos, goarch)]
	return a, ok
}

// artifact is a resolved download job: one URL, one expected size, one expected
// hash. Runtime and model differ only in these fields, which is why the
// download, verify and resume code below is written once.
type artifact struct {
	label string // shown to the user in progress output
	url   string
	file  string // base name under tmp/, plus ".part" while incomplete
	bytes int64
	sum   string
}

// kind is "zip" or "tar.gz", decided by the file name rather than the platform
// so a future platform that changes format needs no code change here.
func (a artifact) kind() string {
	if path.Ext(a.file) == ".zip" {
		return "zip"
	}
	return "tar.gz"
}

func runtimeArtifact(goos, goarch string) (artifact, error) {
	a, ok := runtimeAsset(goos, goarch)
	if !ok {
		return artifact{}, fmt.Errorf("%w: %s", ErrUnsupported, platformKey(goos, goarch))
	}
	return artifact{
		label: "llama.cpp " + RuntimeTag + " · " + a.Label,
		url:   runtimeOrigin + a.File,
		file:  a.File,
		bytes: a.Bytes,
		sum:   a.SHA,
	}, nil
}

func modelArtifact() artifact {
	return artifact{
		label: ModelLabel,
		url:   modelOrigin + ModelFileName,
		file:  ModelFileName,
		bytes: ModelDownloadBytes,
		sum:   ModelSHA256,
	}
}

// serverBinName is the llama-server executable's name on this platform.
func serverBinName(goos string) string {
	if goos == "windows" {
		return "llama-server.exe"
	}
	return "llama-server"
}

// thisOS and thisArch exist so tests can exercise the platform branches of pure
// functions without pretending to run elsewhere.
func thisOS() string   { return runtime.GOOS }
func thisArch() string { return runtime.GOARCH }
