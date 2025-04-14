package main

import (
	"context"
	"runtime/debug"

	"github.com/spf13/cobra"

	"github.com/codeasashu/HookRelay/cmd"
)

// Version is dynamically set by the toolchain or overridden by the Makefile.
var Version = "DEV"

func main() {
	if Version == "DEV" {
		if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "(devel)" {
			Version = info.Main.Version
		}
	}

	cobra.CheckErr(cmd.NewCLI(Version).ExecuteContext(context.Background()))
}
