package workflow

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const fileName = "WORKFLOW.md"

var errUnclosedFrontMatter = errors.New("workflow front matter is missing closing delimiter")

type Definition struct {
	Path           string
	Config         map[string]any
	Prompt         string
	PromptTemplate string
	HasFrontMatter bool
}

func DefaultPath() string {
	return filepath.Join(".", fileName)
}

func Load(path string) (Definition, error) {
	workflowPath := strings.TrimSpace(path)
	if workflowPath == "" {
		workflowPath = DefaultPath()
	}

	absPath, err := filepath.Abs(workflowPath)
	if err != nil {
		return Definition{}, fmt.Errorf("resolve workflow path: %w", err)
	}

	content, err := os.ReadFile(absPath)
	if err != nil {
		return Definition{}, fmt.Errorf("read workflow file: %w", err)
	}

	definition, err := Parse(absPath, string(content))
	if err != nil {
		return Definition{}, err
	}

	return definition, nil
}

func Parse(path string, content string) (Definition, error) {
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")

	config := map[string]any{}
	promptLines := lines
	hasFrontMatter := false

	if len(lines) > 0 && strings.TrimSpace(lines[0]) == "---" {
		hasFrontMatter = true
		closing := -1

		for index := 1; index < len(lines); index++ {
			if strings.TrimSpace(lines[index]) == "---" {
				closing = index
				break
			}
		}

		if closing == -1 {
			return Definition{}, errUnclosedFrontMatter
		}

		frontMatter := strings.Join(lines[1:closing], "\n")
		promptLines = lines[closing+1:]

		if strings.TrimSpace(frontMatter) != "" {
			if err := yaml.Unmarshal([]byte(frontMatter), &config); err != nil {
				return Definition{}, fmt.Errorf("decode workflow front matter: %w", err)
			}

			if config == nil {
				config = map[string]any{}
			}
		}
	}

	prompt := strings.TrimSpace(strings.Join(promptLines, "\n"))

	return Definition{
		Path:           path,
		Config:         config,
		Prompt:         prompt,
		PromptTemplate: prompt,
		HasFrontMatter: hasFrontMatter,
	}, nil
}
