package main

import (
	"fmt"
	"runtime"

	"github.com/alecthomas/kong"

	"github.com/wapc/cli/pkg/commands"
)

var version = "edge"

var cli struct {
	// Install installs a module into the module directory.
	Install commands.InstallCmd `cmd:"" help:"Install a module."`
	// Generate generates code driven by a configuration file.
	Generate commands.GenerateCmd `cmd:"" help:"Generate code from a configuration file."`
	// New creates a new project from a template.
	New commands.NewCmd `cmd:"" help:"Creates a new project from a template."`
	// Upgrade reinstalls the base module dependencies.
	Upgrade commands.UpgradeCmd `cmd:"" help:"Upgrades to the latest base modules dependencies."`
	// Version prints out the version of this program and runtime info.
	Version versionCmd `cmd:""`
}

func main() {
	ctx := kong.Parse(&cli)
	// Call the Run() method of the selected parsed command.
	err := ctx.Run(&commands.Context{})
	ctx.FatalIfErrorf(err)
}

type versionCmd struct{}

func (c *versionCmd) Run() error {
	fmt.Printf("wapc version %s %s/%s\n", version, runtime.GOOS, runtime.GOARCH)
	return nil
}
