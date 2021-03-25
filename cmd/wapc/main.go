package main

import (
	"github.com/alecthomas/kong"

	"github.com/wapc/cli-go/pkg/commands"
)

var cli struct {
	// Install installs a module into the module directory.
	Install commands.InstallCmd `cmd:"" help:"Install a module."`
	// Generate generates code driven by a configuration file.
	Generate commands.GenerateCmd `cmd:"" help:"Generate code from a configuration file."`
	// New creates a new project from a template.
	New commands.NewCmd `cmd:"" help:"Creates a new project from a template."`
}

func main() {
	ctx := kong.Parse(&cli)
	// Call the Run() method of the selected parsed command.
	err := ctx.Run(&commands.Context{})
	ctx.FatalIfErrorf(err)
}
