package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mitchellh/go-homedir"
)

var baseDependencies = map[string][]string{
	"@wapc/widl": {
		filepath.Join("src", "@wapc", "widl"),
	},
	"@wapc/widl-codegen": {
		filepath.Join("src", "@wapc", "widl-codegen"),
		filepath.Join("templates", "@wapc", "widl-codegen"),
	},
}

func getHomeDirectory() (string, error) {
	home, err := homedir.Dir()
	if err != nil {
		return "", err
	}
	home, err = homedir.Expand(home)
	if err != nil {
		return "", err
	}

	wapcHome := filepath.Join(home, ".wapc")
	srcDir := filepath.Join(wapcHome, "src")
	templatesDir := filepath.Join(wapcHome, "templates")

	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		if err = os.MkdirAll(srcDir, 0700); err != nil {
			return "", err
		}
	}

	if _, err := os.Stat(templatesDir); os.IsNotExist(err) {
		if err = os.MkdirAll(templatesDir, 0700); err != nil {
			return "", err
		}
	}

	missing := make(map[string]struct{}, len(baseDependencies))
	for dependency, checks := range baseDependencies {
		for _, check := range checks {
			if _, err := os.Stat(filepath.Join(wapcHome, check)); os.IsNotExist(err) {
				missing[dependency] = struct{}{}
			}
		}
	}

	if len(missing) > 0 {
		fmt.Println("Installing base dependencies...")
		for dependency := range missing {
			cmd := InstallCmd{
				Location: dependency,
			}
			if err = cmd.doRun(&Context{}, wapcHome); err != nil {
				return "", err
			}
		}
	}

	return wapcHome, nil
}
