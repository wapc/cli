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
	wapcHome, err := ensureHomeDirectory()
	if err != nil {
		return "", err
	}

	err = checkDependencies(wapcHome, false)

	return wapcHome, err
}

const tsconfigContents = `{
  "compilerOptions": {
    "module": "commonjs",
    "target": "esnext",
    "baseUrl": ".",
    "lib": [      
      "esnext"
    ],
    "outDir": "../dist"
  }
}
`

func ensureHomeDirectory() (string, error) {
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
	definitionsDir := filepath.Join(wapcHome, "definitions")

	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		if err = os.MkdirAll(srcDir, 0700); err != nil {
			return "", err
		}
	}

	// Create tsconfig.json inside the src directory for editing inside an IDE.
	tsconfigJSON := filepath.Join(srcDir, "tsconfig.json")
	if _, err := os.Stat(tsconfigJSON); os.IsNotExist(err) {
		if err = os.WriteFile(tsconfigJSON, []byte(tsconfigContents), 0644); err != nil {
			return "", err
		}
	}

	if _, err := os.Stat(templatesDir); os.IsNotExist(err) {
		if err = os.MkdirAll(templatesDir, 0700); err != nil {
			return "", err
		}
	}

	if _, err := os.Stat(definitionsDir); os.IsNotExist(err) {
		if err = os.MkdirAll(definitionsDir, 0700); err != nil {
			return "", err
		}
	}

	return wapcHome, nil
}

func checkDependencies(wapcHome string, forceDownload bool) error {
	missing := make(map[string]struct{}, len(baseDependencies))
	for dependency, checks := range baseDependencies {
		for _, check := range checks {
			if forceDownload {
				missing[dependency] = struct{}{}
			} else if _, err := os.Stat(filepath.Join(wapcHome, check)); os.IsNotExist(err) {
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
			if err := cmd.doRun(&Context{}, wapcHome); err != nil {
				return err
			}
		}
	}

	return nil
}
