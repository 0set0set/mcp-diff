package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/0set0set/mcp-diff/internal/config"
	"github.com/0set0set/mcp-diff/internal/diff"
	"github.com/0set0set/mcp-diff/internal/output"
)

const usage = "Usage: mcp-diff [--json] [--format auto|claude|cursor|vscode|gemini] BEFORE AFTER"

var version = "dev"

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

type options struct {
	paths      []string
	format     config.Format
	jsonOutput bool
	help       bool
	version    bool
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	options, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "mcp-diff: %v\n%s\n", err, usage)
		return 2
	}
	if options.help {
		fmt.Fprintln(stdout, usage)
		return 0
	}
	if options.version {
		fmt.Fprintf(stdout, "mcp-diff %s\n", version)
		return 0
	}

	beforeData, err := readInput(options.paths[0], stdin)
	if err != nil {
		fmt.Fprintf(stderr, "mcp-diff: %v\n", err)
		return 2
	}

	afterData, err := readInput(options.paths[1], stdin)
	if err != nil {
		fmt.Fprintf(stderr, "mcp-diff: %v\n", err)
		return 2
	}

	format, err := resolveFormat(beforeData, afterData, options.format)
	if err != nil {
		fmt.Fprintf(stderr, "mcp-diff: %v\n", err)
		return 2
	}
	before, err := config.Load(beforeData, format)
	if err != nil {
		fmt.Fprintf(stderr, "mcp-diff: parse %q: %v\n", options.paths[0], err)
		return 2
	}
	after, err := config.Load(afterData, format)
	if err != nil {
		fmt.Fprintf(stderr, "mcp-diff: parse %q: %v\n", options.paths[1], err)
		return 2
	}

	result := diff.Compute(before, after, format)
	if options.jsonOutput {
		err = output.WriteJSON(stdout, result)
	} else {
		err = output.WriteText(stdout, result)
	}
	if err != nil {
		fmt.Fprintf(stderr, "mcp-diff: write output: %v\n", err)
		return 2
	}

	if result.HasChanges() {
		return 1
	}
	return 0
}

func parseArgs(args []string) (options, error) {
	result := options{format: config.FormatAuto}
	flagsEnabled := true
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch {
		case flagsEnabled && arg == "--":
			flagsEnabled = false
		case flagsEnabled && arg == "--json":
			result.jsonOutput = true
		case flagsEnabled && (arg == "-h" || arg == "--help"):
			result.help = true
		case flagsEnabled && arg == "--version":
			result.version = true
		case flagsEnabled && arg == "--format":
			index++
			if index >= len(args) {
				return options{}, fmt.Errorf("--format requires a value")
			}
			format, err := config.ParseFormat(args[index])
			if err != nil {
				return options{}, err
			}
			result.format = format
		case flagsEnabled && strings.HasPrefix(arg, "--format="):
			format, err := config.ParseFormat(strings.TrimPrefix(arg, "--format="))
			if err != nil {
				return options{}, err
			}
			result.format = format
		case flagsEnabled && strings.HasPrefix(arg, "-") && arg != "-":
			return options{}, fmt.Errorf("unknown option %q", arg)
		default:
			result.paths = append(result.paths, arg)
		}
	}

	if result.help || result.version {
		return result, nil
	}
	if len(result.paths) != 2 {
		return options{}, fmt.Errorf("expected two input paths, got %d", len(result.paths))
	}
	if result.paths[0] == "-" && result.paths[1] == "-" {
		return options{}, fmt.Errorf("standard input can be used for only one input")
	}
	return result, nil
}

func readInput(path string, stdin io.Reader) ([]byte, error) {
	if path == "-" {
		data, err := io.ReadAll(stdin)
		if err != nil {
			return nil, fmt.Errorf("read standard input: %w", err)
		}
		return data, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %q: %w", path, err)
	}
	return data, nil
}

func resolveFormat(before, after []byte, requested config.Format) (config.Format, error) {
	if requested != config.FormatAuto {
		return requested, nil
	}
	beforeFormat, err := config.Detect(before)
	if err != nil {
		return "", fmt.Errorf("detect before format: %w", err)
	}
	afterFormat, err := config.Detect(after)
	if err != nil {
		return "", fmt.Errorf("detect after format: %w", err)
	}
	if beforeFormat == afterFormat {
		return beforeFormat, nil
	}
	if beforeFormat == config.FormatClaude {
		return afterFormat, nil
	}
	if afterFormat == config.FormatClaude {
		return beforeFormat, nil
	}
	return "", fmt.Errorf("input formats differ: before is %s, after is %s", beforeFormat, afterFormat)
}
