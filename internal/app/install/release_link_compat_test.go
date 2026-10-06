package install

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestLinkReleaseNames(t *testing.T) {
	if got := releaseAssetName("v2.2.0", "darwin", "arm64"); got != "codex-feishu-link_2.2.0_darwin_arm64.tar.gz" {
		t.Fatalf("asset name = %q", got)
	}
}

func TestLinkReleaseFallsBackToLegacyArchiveOnNotFound(t *testing.T) {
	version := "v2.2.0"
	legacyPackage := fmt.Sprintf("codex-feishu-relay_2.2.0_%s_%s", runtime.GOOS, runtime.GOARCH)
	legacyBinary := "codex-feishu-relay"
	if runtime.GOOS == "windows" {
		legacyBinary += ".exe"
	}
	archive := filepath.Join(t.TempDir(), legacyPackage+releaseArchiveSuffix(runtime.GOOS))
	writePlatformReleaseArchive(t, archive, legacyPackage, legacyBinary, "legacy-payload", runtime.GOOS)
	var requested []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested = append(requested, filepath.Base(r.URL.Path))
		if filepath.Base(r.URL.Path) != filepath.Base(archive) {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, archive)
	}))
	defer server.Close()
	binary, err := EnsureReleaseBinary(context.Background(), ReleaseBinaryOptions{
		Version: version, BaseURL: server.URL, VersionsRoot: t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(requested) != 2 || requested[0] != releaseAssetName(version, runtime.GOOS, runtime.GOARCH) || requested[1] != filepath.Base(archive) {
		t.Fatalf("unexpected fallback requests: %v", requested)
	}
	data, err := os.ReadFile(binary)
	if err != nil || string(data) != "legacy-payload" {
		t.Fatalf("legacy payload unreadable: %q, %v", data, err)
	}
}

func TestLinkReleaseDoesNotFallbackOnServerError(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	_, err := EnsureReleaseBinary(context.Background(), ReleaseBinaryOptions{
		Version: "v2.2.0", BaseURL: server.URL, VersionsRoot: t.TempDir(),
	})
	if err == nil || requests != 1 {
		t.Fatalf("server failure must not trigger legacy fallback: requests=%d err=%v", requests, err)
	}
}
