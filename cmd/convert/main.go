package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"nodeforge/internal/model"
	"nodeforge/internal/parser"
	"nodeforge/internal/renderer"
	"nodeforge/internal/validate"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		inputShort         string
		inputLong          string
		formatShort        string
		formatLong         string
		outputShort        string
		outputLong         string
		pretty             bool
		group              string
		server             string
		serverFromFilename bool
	)

	flag.StringVar(&inputShort, "i", "", "input file or directory path")
	flag.StringVar(&inputLong, "input", "", "input file or directory path")
	flag.StringVar(&formatShort, "f", "", "output format: clash, singbox, links, v2rayn, all")
	flag.StringVar(&formatLong, "format", "", "output format: clash, singbox, links, v2rayn, all")
	flag.StringVar(&outputShort, "o", "", "output file or output directory")
	flag.StringVar(&outputLong, "output", "", "output file or output directory")
	flag.BoolVar(&pretty, "pretty", false, "pretty print json output")
	flag.StringVar(&group, "group", "", "default group name")
	flag.StringVar(&server, "server", "", "default server address used when server-side configs do not include an external host")
	flag.BoolVar(&serverFromFilename, "server-from-filename", false, "when input is a directory, derive the default server domain from each config filename")
	flag.Parse()

	input := firstNonEmpty(inputLong, inputShort)
	output := firstNonEmpty(outputLong, outputShort)
	if input == "" {
		return errors.New("missing required -i/--input")
	}
	if output == "" {
		return errors.New("missing required -o/--output")
	}
	if server != "" && serverFromFilename {
		return errors.New("--server and --server-from-filename cannot be used together")
	}

	inputInfo, err := os.Stat(input)
	if err != nil {
		return fmt.Errorf("stat input path: %w", err)
	}
	if serverFromFilename && !inputInfo.IsDir() {
		return errors.New("--server-from-filename requires -i/--input to be a directory")
	}

	format, err := renderer.ParseFormat(firstNonEmpty(formatLong, formatShort))
	if err != nil {
		return err
	}

	parseResult, err := parser.ParsePath(input, parser.Options{
		DefaultServer:      server,
		ServerFromFilename: serverFromFilename,
	})
	if err != nil {
		return err
	}

	defaultGroup := firstNonEmpty(group, parseResult.SourceGroup, "Proxy")
	validNodes, invalidErrors, invalidCount := normalizeAndValidate(parseResult.Nodes, defaultGroup)
	issues := append(parseResult.Errors, invalidErrors...)
	total := len(parseResult.Nodes) + len(parseResult.Errors)
	failed := len(parseResult.Errors) + invalidCount

	if err := writeOutputs(validNodes, format, output, defaultGroup, pretty); err != nil {
		return err
	}

	reportStats(total, len(validNodes), failed, issues)
	return nil
}

func normalizeAndValidate(nodes []model.Node, defaultGroup string) ([]model.Node, []error, int) {
	validNodes := make([]model.Node, 0, len(nodes))
	var errs []error
	invalidCount := 0
	for index, node := range nodes {
		node.Normalize(defaultGroup)
		nodeErrs := validate.Node(node)
		if len(nodeErrs) > 0 {
			invalidCount++
			for _, nodeErr := range nodeErrs {
				errs = append(errs, fmt.Errorf("node %d (%s): %w", index+1, node.Name, nodeErr))
			}
			continue
		}
		validNodes = append(validNodes, node)
	}
	return validNodes, errs, invalidCount
}

func writeOutputs(nodes []model.Node, format renderer.Format, output, group string, pretty bool) error {
	if len(nodes) == 0 {
		return errors.New("no valid nodes to render")
	}

	switch format {
	case renderer.FormatAll:
		if err := os.MkdirAll(output, 0o755); err != nil {
			return fmt.Errorf("create output directory: %w", err)
		}
		for _, item := range []renderer.Format{renderer.FormatClash, renderer.FormatSingBox, renderer.FormatLinks, renderer.FormatV2RayN} {
			target := filepath.Join(output, renderer.DefaultFilename(item))
			if err := writeOne(nodes, item, target, group, pretty); err != nil {
				return err
			}
		}
		return nil
	default:
		target, err := resolveSingleOutputPath(output, format)
		if err != nil {
			return err
		}
		return writeOne(nodes, format, target, group, pretty)
	}
}

func writeOne(nodes []model.Node, format renderer.Format, output, group string, pretty bool) error {
	var (
		data []byte
		err  error
	)

	switch format {
	case renderer.FormatClash:
		data, err = renderer.RenderClash(nodes, group)
	case renderer.FormatSingBox:
		data, err = renderer.RenderSingBox(nodes, group, pretty)
	case renderer.FormatLinks:
		data, err = renderer.RenderLinks(nodes)
	case renderer.FormatV2RayN:
		data, err = renderer.RenderV2RayNSubscription(nodes)
	default:
		err = fmt.Errorf("unsupported format %q", format)
	}
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return fmt.Errorf("create output parent directory: %w", err)
	}
	if err := os.WriteFile(output, data, 0o644); err != nil {
		return fmt.Errorf("write output file %s: %w", output, err)
	}
	return nil
}

func resolveSingleOutputPath(output string, format renderer.Format) (string, error) {
	info, err := os.Stat(output)
	if err == nil && info.IsDir() {
		return filepath.Join(output, renderer.DefaultFilename(format)), nil
	}
	if err == nil {
		return output, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("inspect output path: %w", err)
	}
	if filepath.Ext(output) == "" {
		return filepath.Join(output, renderer.DefaultFilename(format)), nil
	}
	return output, nil
}

func reportStats(total, valid, failed int, issues []error) {
	for _, issue := range issues {
		fmt.Fprintf(os.Stderr, "warning: %v\n", issue)
	}
	fmt.Fprintf(os.Stderr, "summary: total=%d valid=%d failed=%d\n", total, valid, failed)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
