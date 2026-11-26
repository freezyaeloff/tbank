package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func readFile(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(name) // #nosec G304 -- имя файла из временного каталога теста.
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func runProcess(t *testing.T, wantCode int, program string, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), program, args...) // #nosec G204 -- тест запускает собранный бинарник.
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatal(err)
		}
	}
	if code := cmd.ProcessState.ExitCode(); code != wantCode {
		t.Fatalf("exit = %d, want %d; stderr = %q", code, wantCode, stderr.String())
	}
	return stderr.String()
}

func TestCLI(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(dir, "incident-radar")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	if out, err := exec.CommandContext(t.Context(), "go", "build", "-o", binary, ".").CombinedOutput(); err != nil { // #nosec G204 -- сборка тестового CLI.
		t.Fatalf("go build: %v\n%s", err, out)
	}
	a, b := filepath.Join(dir, "a.log"), filepath.Join(dir, "b.log")
	putFile(t, a, exampleLine+"\nmalformed\n")
	putFile(t, b, strings.Replace(exampleLine, "304 0", "200 100", 1))

	t.Run("JSON from glob", func(t *testing.T) {
		output := filepath.Join(dir, "result.json")
		stderr := runProcess(t, 0, binary, "-p", filepath.Join(dir, "*.log"), "-o", output)
		if stderr != "WARN: skipped malformed log line\n" {
			t.Errorf("stderr = %q", stderr)
		}
		var got map[string]any
		if err := json.Unmarshal([]byte(readFile(t, output)), &got); err != nil {
			t.Fatal(err)
		}
		want := map[string]any{
			"files": []any{a, b}, "totalRequestsCount": 2.0,
			"responseSizeInBytes": map[string]any{"average": 50.0, "max": 100.0, "p95": 100.0},
			"resources":           []any{map[string]any{"resource": "/downloads/product_1", "count": 2.0}},
			"responseCodes":       []any{map[string]any{"code": 200.0, "count": 1.0}, map[string]any{"code": 304.0, "count": 1.0}},
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("report = %v, want %v", got, want)
		}
	})
	t.Run("Markdown from URL", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, strings.ReplaceAll(exampleLine, "/downloads/product_1", "/a|b"))
		}))
		defer server.Close()
		url := server.URL + "/access.txt"
		output := filepath.Join(dir, "result.md")
		runProcess(t, 0, binary, "--path", url, "--format", "markdown", "--output", output, "--from", "2015-05-17")
		body := readFile(t, output)
		for _, part := range []string{
			"# Incident Radar", "| " + url + " |", "| Всего запросов | 1 |", "| Средний размер ответа, байт | 0.00 |",
			"| Максимум, байт | 0 |", "| p95, байт | 0 |", "| /a&#124;b | 1 |", "| 304 Not Modified | 1 |",
		} {
			if !strings.Contains(body, part) {
				t.Errorf("report lacks %q", part)
			}
		}
	})
	t.Run("usage errors", func(t *testing.T) {
		output := filepath.Join(dir, "never.json")
		server := httptest.NewServer(http.NotFoundHandler())
		defer server.Close()
		for _, tc := range []struct {
			name string
			args []string
		}{
			{"missing path", []string{"-o", output}},
			{"format", []string{"-p", a, "-o", output, "-f", "xml"}},
			{"dates", []string{"-p", a, "-o", output, "--from", "2025-03-01", "--to", "2025-03-01"}},
			{"extension", []string{"-p", a, "-o", output, "-f", "markdown"}},
			{"missing file", []string{"-p", filepath.Join(dir, "missing.txt"), "-o", output}},
			{"empty glob", []string{"-p", filepath.Join(dir, "*.txt"), "-o", output}},
			{"HTTP 404", []string{"-p", server.URL + "/missing.log", "-o", output}},
		} {
			t.Run(tc.name, func(t *testing.T) {
				if runProcess(t, 2, binary, tc.args...) == "" {
					t.Fatal("missing error message")
				}
				if _, err := os.Stat(output); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("stat output: %v", err)
				}
			})
		}
	})
	t.Run("existing report", func(t *testing.T) {
		output := filepath.Join(dir, "existing.json")
		putFile(t, output, "original")
		runProcess(t, 2, binary, "-p", a, "-o", output)
		if got := readFile(t, output); got != "original" {
			t.Fatalf("output = %q", got)
		}
	})
	t.Run("write failure", func(t *testing.T) {
		if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
			t.Skip("requires POSIX file size limit")
		}
		output := filepath.Join(dir, "failed.json")
		// Запрет записи даже одного байта имитирует сбой уже после создания отчёта.
		stderr := runProcess(t, 1, "sh", "-c", `trap '' XFSZ; ulimit -f 0 || exit 99; exec "$@"`, "sh", binary, "-p", b, "-o", output)
		if !strings.Contains(stderr, "запись отчёта") {
			t.Fatalf("stderr = %q", stderr)
		}
		if _, err := os.Stat(output); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("partial report: %v", err)
		}
	})
}

func TestReportCreatedDuringReadIsPreserved(t *testing.T) {
	output := filepath.Join(t.TempDir(), "report.json")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if err := os.WriteFile(output, []byte("created while reading"), 0o600); err != nil {
			t.Error(err)
		}
		_, _ = io.WriteString(w, exampleLine)
	}))
	defer server.Close()

	var stderr bytes.Buffer
	if code := run(t.Context(), []string{"-p", server.URL + "/access.log", "-o", output}, &stderr); code != 2 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr.String())
	}
	if got := readFile(t, output); got != "created while reading" {
		t.Fatalf("overwrote concurrent report: %q", got)
	}
}
