package product

import "testing"

func TestExecutableNames(t *testing.T) {
	for _, tc := range []struct {
		goos, current, legacy string
	}{
		{"linux", "codex-feishu-link", "codex-feishu-relay"},
		{"darwin", "codex-feishu-link", "codex-feishu-relay"},
		{"windows", "codex-feishu-link.exe", "codex-feishu-relay.exe"},
	} {
		t.Run(tc.goos, func(t *testing.T) {
			if got := ExecutableName(tc.goos); got != tc.current {
				t.Fatalf("ExecutableName = %q, want %q", got, tc.current)
			}
			if got := LegacyExecutableName(tc.goos); got != tc.legacy {
				t.Fatalf("LegacyExecutableName = %q, want %q", got, tc.legacy)
			}
		})
	}
}
