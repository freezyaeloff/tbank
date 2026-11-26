package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func putFile(t *testing.T, name, content string) {
	t.Helper()
	if err := os.WriteFile(name, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestInvalidArgumentsAndSources(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "input.log")
	output := filepath.Join(dir, "report.json")
	putFile(t, input, "line")
	existing := filepath.Join(dir, "existing.json")
	putFile(t, existing, "original")
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()

	for _, tc := range []struct {
		name string
		args []string
	}{
		{"missing path", []string{"-o", output}},
		{"missing output", []string{"-p", input}},
		{"unknown format", []string{"-p", input, "-o", output, "-f", "xml"}},
		{"wrong extension", []string{"-p", input, "-o", output, "-f", "markdown"}},
		{"invalid date", []string{"-p", input, "-o", output, "--to", "2025-02-30"}},
		{"equal bounds", []string{"-p", input, "-o", output, "--from", "2025-02-01", "--to", "2025-02-01"}},
		{"reversed bounds", []string{"-p", input, "-o", output, "--from", "2025-02-02", "--to", "2025-02-01"}},
		{"missing file", []string{"-p", filepath.Join(dir, "missing.txt"), "-o", output}},
		{"empty glob", []string{"-p", filepath.Join(dir, "*.txt"), "-o", output}},
		{"unsupported input", []string{"-p", existing, "-o", output}},
		{"HTTP 404", []string{"-p", server.URL + "/404.txt", "-o", output}},
		{"existing report", []string{"-p", input, "-o", existing}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var stderr bytes.Buffer
			if code := run(t.Context(), tc.args, &stderr); code != 2 || stderr.Len() == 0 {
				t.Fatalf("exit = %d, stderr = %q", code, stderr.String())
			}
		})
	}
}

func TestReadSources(t *testing.T) {
	dir := t.TempDir()
	a, b := filepath.Join(dir, "a.txt"), filepath.Join(dir, "b.log")
	putFile(t, a, "first\r\nsecond")
	putFile(t, b, "third\n")
	putFile(t, filepath.Join(dir, "other.csv"), "ignored")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "remote\n")
	}))
	defer server.Close()
	url := server.URL + "/access.txt?part=1"
	for _, tc := range []struct {
		name, path   string
		files, lines []string
	}{
		{"file", a, []string{a}, []string{"first", "second"}},
		{"glob", filepath.Join(dir, "*"), []string{a, b}, []string{"first", "second", "third"}},
		{"url", url, []string{url}, []string{"remote"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			files, err := sourceNames(tc.path)
			if err != nil || !slices.Equal(files, tc.files) {
				t.Fatalf("files = %v, err = %v", files, err)
			}
			var lines []string
			err = readSources(t.Context(), server.Client(), files, func(s string) {
				lines = append(lines, s)
			})
			if err != nil || !slices.Equal(lines, tc.lines) {
				t.Fatalf("lines = %q, err = %v", lines, err)
			}
		})
	}
}

type readerFunc func([]byte) (int, error)

func (f readerFunc) Read(p []byte) (int, error) { return f(p) }

func TestReadLinesIsStreaming(t *testing.T) {
	var lines []string
	last := strings.Repeat("a", 100_000)
	r := io.MultiReader(strings.NewReader("first\n"), readerFunc(func([]byte) (int, error) {
		if !slices.Equal(lines, []string{"first"}) {
			t.Fatal("first line was not processed before requesting more data")
		}
		return 0, io.EOF
	}), strings.NewReader(last))
	err := readLines(r, func(s string) { lines = append(lines, s) })
	if err != nil || !slices.Equal(lines, []string{"first", last}) {
		t.Fatalf("long final line was not processed, err = %v", err)
	}
}
