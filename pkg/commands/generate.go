package commands

import (
	_ "embed"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/evanw/esbuild/pkg/api"
	"gopkg.in/yaml.v3"
	"rogchap.com/v8go"

	"github.com/wapc/cli/pkg/js"
)

type Context struct{}

type GenerateCmd struct {
	Config string `arg:"" help:"The code generation configuration file" type:"existingfile"`

	prettier *js.JS
	once     sync.Once
}

type Config struct {
	Schema    string                 `json:"schema" yaml:"schema"`
	Config    map[string]interface{} `json:"config,omitempty" yaml:"config,omitempty"`
	Generates map[string]Target      `json:"generates" yaml:"generates"`
}

type Target struct {
	Module       string                 `json:"module" yaml:"module"`
	VisitorClass string                 `json:"visitorClass" yaml:"visitorClass"`
	IfNotExists  bool                   `json:"ifNotExists,omitempty" yaml:"ifNotExists,omitempty"`
	Config       map[string]interface{} `json:"config,omitempty" yaml:"config,omitempty"`
}

const generateTemplate = `import { parse } from "@wapc/widl";
import { Context, Writer } from "@wapc/widl/ast";
import { {{visitorClass}} } from "{{module}}";

function resolver(location, from) {
  const source = resolverCallback(location, from);
  if (source.startsWith("error: ")) {
    throw source.substring(7);
  }
  return source;
}

export function generate(widl, config) {
  const doc = parse(widl, resolver);
  const context = new Context(config);

  const writer = new Writer();
  const visitor = new {{visitorClass}}(writer);
  doc.accept(context, visitor);
  let source = writer.string();

  return source;
}

js_exports["generate"] = generate;`

func (c *GenerateCmd) Run(ctx *Context) error {
	defer func() {
		if c.prettier != nil {
			c.prettier.Dispose()
		}
	}()

	configBytes, err := readFile(c.Config)
	if err != nil {
		return err
	}

	configs := strings.Split(string(configBytes), "---")
	for _, config := range configs {
		if err := c.generate(config); err != nil {
			return err
		}
	}

	return nil
}

func (c *GenerateCmd) generate(configYAML string) error {
	var config Config
	if err := yaml.Unmarshal([]byte(configYAML), &config); err != nil {
		return err
	}

	schemaBytes, err := readFile(config.Schema)
	if err != nil {
		return err
	}
	schema := string(schemaBytes)

	homeDir, err := getHomeDirectory()
	if err != nil {
		return err
	}
	srcDir := filepath.Join(homeDir, "src")

	for filename, target := range config.Generates {
		if target.IfNotExists {
			_, err := os.Stat(filename)
			if err != nil && !os.IsNotExist(err) {
				return err
			}
			if err == nil {
				fmt.Printf("Skipping %s...\n", filename)
				continue
			}
		}

		// Merge global config into target config
		if target.Config == nil && config.Config != nil {
			target.Config = make(map[string]interface{}, len(config.Config))
		}
		for k, v := range config.Config {
			if _, exists := target.Config[k]; !exists {
				target.Config[k] = v
			}
		}

		fmt.Printf("Generating %s...\n", filename)
		generateTS := generateTemplate
		generateTS = strings.Replace(generateTS, "{{module}}", target.Module, 1)
		generateTS = strings.Replace(generateTS, "{{visitorClass}}", target.VisitorClass, -1)

		result := api.Build(api.BuildOptions{
			Stdin: &api.StdinOptions{
				Contents:   generateTS,
				Sourcefile: "generate.ts",
				ResolveDir: srcDir,
			},
			Bundle:    true,
			NodePaths: []string{srcDir},
			LogLevel:  api.LogLevelInfo,
		})
		if len(result.Errors) > 0 {
			return fmt.Errorf("esbuild returned errors: %v", result.Errors)
		}
		if len(result.OutputFiles) != 1 {
			return errors.New("esbuild did not produce exactly 1 output file")
		}

		bundle := string(result.OutputFiles[0].Contents)

		definitionsDir := filepath.Join(homeDir, "definitions")

		resolverCallback := func(info *v8go.FunctionCallbackInfo) *v8go.Value {
			iso, err := info.Context().Isolate()
			if err != nil {
				return nil
			}

			if len(info.Args()) < 1 {
				value, _ := v8go.NewValue(iso, "error: resolve: invalid arguments")
				return value
			}

			location := info.Args()[0].String()

			loc := filepath.Join(definitionsDir, filepath.Join(strings.Split(location, "/")...))
			if filepath.Ext(loc) != ".widl" {
				widlLoc := loc + ".widl"
				found := false
				stat, err := os.Stat(widlLoc)
				if err == nil && !stat.IsDir() {
					found = true
					loc = widlLoc
				}

				if !found {
					stat, err := os.Stat(loc)
					if err != nil {
						value, _ := v8go.NewValue(iso, fmt.Sprintf("error: %v", err))
						return value
					}
					if stat.IsDir() {
						loc = filepath.Join(loc, "index.widl")
					} else {
						loc += ".widl"
					}
				}
			}

			data, err := os.ReadFile(loc)
			if err != nil {
				value, _ := v8go.NewValue(iso, fmt.Sprintf("error: %v", err))
				return value
			}

			value, _ := v8go.NewValue(iso, string(data))
			return value
		}

		j, err := js.Compile(bundle, map[string]v8go.FunctionCallback{
			"resolverCallback": resolverCallback,
		})
		if err != nil {
			return err
		}
		defer j.Dispose()

		if target.Config == nil {
			target.Config = map[string]interface{}{}
		}
		res, err := j.Invoke("generate", schema, target.Config)
		if err != nil {
			if jserr, ok := err.(*v8go.JSError); ok {
				jserr.Message = strings.TrimPrefix(jserr.Message, "Error: ")
			}
			return err
		}

		source := res.(string)
		ext := filepath.Ext(filename)
		switch ext {
		case ".ts":
			source, err = c.formatTypeScript(source)
			if err != nil {
				return err
			}
		}

		dir := filepath.Dir(filename)
		if dir != "" {
			if err = os.MkdirAll(dir, 0777); err != nil {
				return err
			}
		}
		if err = os.WriteFile(filename, []byte(source), 0666); err != nil {
			return err
		}
	}

	// Some CLI-based formatters actually check for types referenced in other files
	// so we must call these after all the files are generated.
	for filename := range config.Generates {
		ext := filepath.Ext(filename)
		switch ext {
		case ".rs":
			fmt.Printf("Formatting %s...\n", filename)
			if err = formatRust(filename); err != nil {
				return err
			}
		case ".go":
			fmt.Printf("Formatting %s...\n", filename)
			if err = formatGolang(filename); err != nil {
				return err
			}
		}
	}

	return nil
}

//go:embed prettier.js
var prettierSource string

func (c *GenerateCmd) formatTypeScript(source string) (string, error) {
	var err error
	c.once.Do(func() {
		c.prettier, err = js.Compile(prettierSource)
	})
	if err != nil {
		return "", err
	}

	res, err := c.prettier.Invoke("formatTypeScript", source)
	if err != nil {
		return "", err
	}

	return res.(string), nil
}

func formatRust(filename string) error {
	cmd := exec.Command("rustfmt", filename)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func formatGolang(filename string) error {
	cmd := exec.Command("gofmt", "-w", filename)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func readFile(file string) ([]byte, error) {
	if strings.HasPrefix(file, "http://") || strings.HasPrefix(file, "https://") {
		resp, err := http.Get(file)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		return io.ReadAll(resp.Body)
	}

	return os.ReadFile(file)
}
