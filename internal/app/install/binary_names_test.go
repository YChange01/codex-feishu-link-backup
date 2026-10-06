package install

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/YChange01/codex-feishu-link/internal/product"
)

func TestInstallProductBinaryRefreshesBothCommandNames(t *testing.T) {
	for _, sourceName := range []string{product.ExecutableName(runtime.GOOS), product.LegacyExecutableName(runtime.GOOS)} {
		t.Run(sourceName, func(t *testing.T) {
			installDir := t.TempDir()
			source := filepath.Join(t.TempDir(), sourceName)
			if err := os.WriteFile(source, []byte("new-payload"), 0o755); err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{product.ExecutableName(runtime.GOOS), product.LegacyExecutableName(runtime.GOOS)} {
				if err := os.WriteFile(filepath.Join(installDir, name), []byte("old-payload"), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			installed, err := installBinary(source, installDir)
			if err != nil {
				t.Fatal(err)
			}
			if installed != filepath.Join(installDir, product.ExecutableName(runtime.GOOS)) {
				t.Fatalf("install state must use canonical command, got %q", installed)
			}
			for _, name := range []string{product.ExecutableName(runtime.GOOS), product.LegacyExecutableName(runtime.GOOS)} {
				data, err := os.ReadFile(filepath.Join(installDir, name))
				if err != nil || string(data) != "new-payload" {
					t.Fatalf("%s is stale: %q %v", name, data, err)
				}
			}
		})
	}
}
