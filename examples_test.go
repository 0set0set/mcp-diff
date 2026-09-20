package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExamples(t *testing.T) {
	t.Parallel()

	directories, err := filepath.Glob(filepath.Join("examples", "*"))
	if err != nil {
		t.Fatalf("Glob() error = %v", err)
	}
	if len(directories) == 0 {
		t.Fatal("no example directories found")
	}

	for _, directory := range directories {
		directory := directory
		info, err := os.Stat(directory)
		if err != nil || !info.IsDir() {
			continue
		}
		t.Run(filepath.Base(directory), func(t *testing.T) {
			t.Parallel()

			format := strings.SplitN(filepath.Base(directory), "-", 2)[0]
			if format == "no" {
				format = "claude"
			}
			before := filepath.Join(directory, "before.json")
			after := filepath.Join(directory, "after.json")
			expectedExitCode := 1
			if strings.HasPrefix(filepath.Base(directory), "no-changes") {
				expectedExitCode = 0
			}

			assertExampleOutput(
				t,
				[]string{"--format", format, before, after},
				filepath.Join(directory, "expected.txt"),
				expectedExitCode,
			)
			assertExampleOutput(
				t,
				[]string{"--json", "--format", format, before, after},
				filepath.Join(directory, "expected.json"),
				expectedExitCode,
			)
		})
	}
}

func assertExampleOutput(t *testing.T, args []string, expectedPath string, expectedExitCode int) {
	t.Helper()

	expected, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", expectedPath, err)
	}

	var stdout, stderr bytes.Buffer
	exitCode := run(args, strings.NewReader(""), &stdout, &stderr)
	if exitCode != expectedExitCode {
		t.Fatalf("run() exit code = %d, want %d; stderr = %q", exitCode, expectedExitCode, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("run() stderr = %q, want empty", stderr.String())
	}
	if !bytes.Equal(stdout.Bytes(), expected) {
		t.Fatalf("run() output:\n%s\nwant:\n%s", stdout.String(), expected)
	}
}
