package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

func logPath(name string) bool {
	ext := filepath.Ext(name)
	return ext == ".txt" || ext == ".log"
}

func sourceNames(path string) ([]string, error) {
	if strings.Contains(path, "://") {
		u, err := url.Parse(path)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return nil, fmt.Errorf("неверный HTTP URL: %q", path)
		}
		if !logPath(u.Path) {
			return nil, errors.New("ожидается URL файла .log или .txt")
		}
		return []string{path}, nil
	}
	// Проверяем точное имя первым: скобки могут быть частью имени файла.
	info, err := os.Stat(path)
	if err == nil {
		if !info.Mode().IsRegular() || !logPath(path) {
			return nil, fmt.Errorf("ожидается обычный файл .log или .txt: %s", path)
		}
		return []string{path}, nil
	}
	if !errors.Is(err, os.ErrNotExist) || !strings.ContainsAny(path, "*?[") {
		return nil, err
	}
	matches, err := filepath.Glob(path)
	if err != nil {
		return nil, fmt.Errorf("glob: %w", err)
	}
	var files []string
	for _, name := range matches {
		if !logPath(name) {
			continue
		}
		info, err := os.Stat(name)
		if err != nil {
			return nil, err
		}
		if info.Mode().IsRegular() {
			files = append(files, name)
		}
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("по маске %q нет файлов .log или .txt", path)
	}
	return files, nil
}

func openSource(ctx context.Context, client *http.Client, name string) (io.ReadCloser, error) {
	if !strings.Contains(name, "://") {
		return os.Open(name) // #nosec G304 -- CLI читает файл, выбранный пользователем.
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, name, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req) // #nosec G704 -- пользователь передаёт URL источника CLI.
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("HTTP %s", resp.Status)
	}
	return resp.Body, nil
}

func readLines(r io.Reader, visit func(string)) error {
	reader := bufio.NewReader(r)
	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			visit(strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r"))
		}
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

func readSources(ctx context.Context, client *http.Client, files []string, visit func(string)) error {
	for _, name := range files {
		r, err := openSource(ctx, client, name)
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		err = readLines(r, visit)
		closeErr := r.Close()
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		if closeErr != nil {
			return fmt.Errorf("%s: %w", name, closeErr)
		}
	}
	return nil
}
