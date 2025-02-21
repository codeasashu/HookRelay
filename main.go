package main

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/codeasashu/HookRelay/cmd"
)

func main() {
	cobra.CheckErr(cmd.NewCLI("1").ExecuteContext(context.Background()))
}
