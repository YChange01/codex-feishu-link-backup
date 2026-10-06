package main

import (
	"os"

	"github.com/YChange01/codex-feishu-link/internal/app/launcher"
)

var version = "dev"
var branch = "dev"

func main() {
	os.Exit(launcher.Main(launcher.Options{
		Args:    os.Args[1:],
		Stdin:   os.Stdin,
		Stdout:  os.Stdout,
		Stderr:  os.Stderr,
		Version: version,
		Branch:  branch,
	}))
}
