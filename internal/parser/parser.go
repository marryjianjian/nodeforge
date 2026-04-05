package parser

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"nodeforge/internal/model"
	"nodeforge/internal/sharelink"
)

type Result struct {
	Nodes       []model.Node
	SourceGroup string
	Errors      []error
}

func ParsePath(path string, opts Options) (Result, error) {
	info, err := os.Stat(path)
	if err != nil {
		return Result{}, fmt.Errorf("stat input path: %w", err)
	}
	if !info.IsDir() {
		return ParseFile(path, opts)
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return Result{}, fmt.Errorf("read input directory: %w", err)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	var result Result
	supportedCount := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		childPath := filepath.Join(path, entry.Name())
		if !isSupportedInputFile(childPath) {
			continue
		}
		supportedCount++
		childResult, err := ParseFile(childPath, opts)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("%s: %w", entry.Name(), err))
			continue
		}
		result.Nodes = append(result.Nodes, childResult.Nodes...)
		for _, childErr := range childResult.Errors {
			result.Errors = append(result.Errors, fmt.Errorf("%s: %w", entry.Name(), childErr))
		}
		if result.SourceGroup == "" && childResult.SourceGroup != "" {
			result.SourceGroup = childResult.SourceGroup
		}
	}
	if supportedCount == 0 {
		return Result{}, fmt.Errorf("no supported input files found in directory %q", path)
	}
	return result, nil
}

func ParseFile(path string, opts Options) (Result, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return Result{}, fmt.Errorf("read input file: %w", err)
	}

	switch strings.ToLower(filepath.Ext(path)) {
	case ".yaml", ".yml":
		return parseStructuredFile(content, true, path, opts)
	case ".json":
		return parseStructuredFile(content, false, path, opts)
	case ".txt":
		return parseLinksFile(content)
	default:
		return Result{}, fmt.Errorf("unsupported input extension %q", filepath.Ext(path))
	}
}

func parseStructuredFile(content []byte, useYAML bool, sourcePath string, opts Options) (Result, error) {
	var (
		doc model.Document
		err error
	)

	if useYAML {
		err = yaml.Unmarshal(content, &doc)
	} else {
		err = json.Unmarshal(content, &doc)
	}
	if err == nil && len(doc.Nodes) > 0 {
		return Result{Nodes: doc.Nodes, SourceGroup: doc.Group}, nil
	}

	var nodes []model.Node
	var nodesErr error
	if useYAML {
		nodesErr = yaml.Unmarshal(content, &nodes)
	} else {
		nodesErr = json.Unmarshal(content, &nodes)
	}
	if nodesErr == nil {
		return Result{Nodes: nodes}, nil
	}

	v2rayResult, v2rayErr := parseV2RayServiceConfig(content, useYAML, sourcePath, opts)
	if v2rayErr == nil {
		return v2rayResult, nil
	}

	if useYAML {
		return Result{}, fmt.Errorf("parse yaml input: %w", v2rayErr)
	}
	return Result{}, fmt.Errorf("parse json input: %w", v2rayErr)
}

func parseLinksFile(content []byte) (Result, error) {
	text := strings.TrimPrefix(string(content), "\uFEFF")
	lines := strings.Split(text, "\n")

	result := Result{}
	for index, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		node, err := sharelink.Parse(line)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("line %d: %w", index+1, err))
			continue
		}
		result.Nodes = append(result.Nodes, node)
	}
	return result, nil
}

func isSupportedInputFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".yaml", ".yml", ".json", ".txt":
		return true
	default:
		return false
	}
}
