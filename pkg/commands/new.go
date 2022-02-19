package commands

import (
	"bytes"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"

	"github.com/tcnksm/go-input"
	"gopkg.in/yaml.v3"
)

type Template struct {
	Name         string     `json:"name" yaml:"name"`
	Description  string     `json:"description" yaml:"description"`
	Variables    []Variable `json:"variables" yaml:"variables"`
	SpecLocation string     `json:"specLocation" yaml:"specLocation"`
}

type Variable struct {
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description" yaml:"description"`
	Prompt      string `json:"prompt" yaml:"prompt"`
	Default     string `json:"default" yaml:"default"`
	Required    bool   `json:"required" yaml:"required"`
	Loop        bool   `json:"loop" yaml:"loop"`
}

type NewCmd struct {
	Template  string            `arg:"" help:"The template for the project to create."`
	Dir       string            `arg:"" help:"The project directory"`
	Spec      string            `type:"existingfile" help:"An optional specification file to copy into the project"`
	Variables map[string]string `arg:"" help:"Variables to pass to the template." optional:""`
}

var firstPartyTranslations = map[string]string{
	"module":         filepath.Join("@wapc", "widl", "module"),
	"assemblyscript": filepath.Join("@wapc", "widl-codegen", "assemblyscript"),
	"rust":           filepath.Join("@wapc", "widl-codegen", "rust"),
	"tinygo":         filepath.Join("@wapc", "widl-codegen", "tinygo"),
}

func (c *NewCmd) Run(ctx *Context) error {
	if strings.Contains(c.Template, "..") {
		return fmt.Errorf("invalid template %s", c.Template)
	}

	homeDir, err := getHomeDirectory()
	if err != nil {
		return err
	}

	if translation, exists := firstPartyTranslations[c.Template]; exists {
		c.Template = translation
	}

	templatePath := filepath.Join(homeDir, "templates", c.Template)
	templateDir, err := os.Stat(templatePath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("template %s is not installed", c.Template)
		}
		return err
	}
	if !templateDir.IsDir() {
		return fmt.Errorf("%s is not a template directory", templatePath)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	projectPath := filepath.Join(cwd, c.Dir)
	if err != nil {
		return err
	}

	fmt.Printf("Creating project directory %s\n", projectPath)
	if err = os.MkdirAll(projectPath, 0777); err != nil {
		return err
	}

	if c.Variables == nil {
		c.Variables = map[string]string{}
	}
	// project name defaults to directory name
	if _, ok := c.Variables["name"]; !ok {
		name := filepath.Base(projectPath)
		c.Variables["name"] = name
	}

	templateBytes, err := os.ReadFile(filepath.Join(templatePath, ".template"))
	if err != nil {
		return err
	}

	var template Template
	if err = yaml.Unmarshal(templateBytes, &template); err != nil {
		return err
	}

	ui := &input.UI{
		Writer: os.Stdout,
		Reader: os.Stdin,
	}

	for _, variable := range template.Variables {
		if _, ok := c.Variables[variable.Name]; !ok {
			value, err := ui.Ask(variable.Prompt, &input.Options{
				Default:   variable.Default,
				Required:  variable.Required,
				Loop:      variable.Loop,
				HideOrder: true,
			})
			if err != nil {
				return err
			}
			c.Variables[variable.Name] = value
		}
	}

	err = c.copy(templatePath, projectPath)
	if err != nil {
		return err
	}

	if c.Spec != "" {
		if template.SpecLocation == "" {
			template.SpecLocation = "spec.widl"
		}
		specFilename := filepath.Join(projectPath, filepath.Clean(template.SpecLocation))
		specBytes, err := os.ReadFile(c.Spec)
		if err != nil {
			return err
		}
		err = os.WriteFile(specFilename, specBytes, 0644)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *NewCmd) copy(source, destination string) error {
	return filepath.Walk(source, func(path string, info os.FileInfo, ferr error) error {
		var relPath string = strings.Replace(path, source, "", 1)
		if relPath == "" {
			return nil
		}

		sourcePath := filepath.Join(source, relPath)
		stat, err := os.Stat(sourcePath)
		if err != nil {
			return err
		}
		if info.IsDir() {
			return os.Mkdir(filepath.Join(destination, relPath), stat.Mode())
		} else {
			base := filepath.Base(sourcePath)
			if base == ".template" || strings.HasPrefix(base, ".git") {
				return nil
			}

			data, err := os.ReadFile(sourcePath)
			if err != nil {
				return err
			}

			if filepath.Ext(relPath) == ".tmpl" {
				tmpl, err := template.New(relPath).Parse(string(data))
				if err != nil {
					return err
				}
				var buf bytes.Buffer
				if err = tmpl.Execute(&buf, c.Variables); err != nil {
					return err
				}

				data = buf.Bytes()
				relPath = relPath[:len(relPath)-5]
			}

			return os.WriteFile(filepath.Join(destination, relPath), data, stat.Mode())
		}
	})
}
