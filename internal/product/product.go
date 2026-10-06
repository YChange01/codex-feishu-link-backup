// Package product separates public product names from compatibility identifiers.
package product

const (
	Name        = "codex-feishu-link"
	DisplayName = "Codex Feishu Link"

	// LegacyNamespace remains the storage and service namespace so upgrades keep
	// the same configuration, credentials, instance bindings, locks, and services.
	LegacyNamespace = "codex-feishu-relay"
)

// ExecutableName returns the current command name for the target platform.
func ExecutableName(goos string) string {
	return executableName(Name, goos)
}

// LegacyExecutableName identifies commands and release packages from before
// the public product rename. Readers use it to preserve upgrade compatibility.
func LegacyExecutableName(goos string) string {
	return executableName(LegacyNamespace, goos)
}

func executableName(name, goos string) string {
	if goos == "windows" {
		return name + ".exe"
	}
	return name
}

// IsCompatibleName recognizes product identities on either side of the rename.
// Version and fingerprint compatibility must still be checked by the caller.
func IsCompatibleName(name string) bool {
	return name == Name || name == LegacyNamespace
}
