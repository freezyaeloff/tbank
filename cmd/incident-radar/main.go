// Package main implements the Incident Radar CLI application.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

func run(ctx context.Context, args []string, stderr io.Writer) int {
	opt, err := parseOptions(args)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 2
	}
	files, err := sourceNames(opt.path)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 2
	}
	client := &http.Client{Timeout: 30 * time.Second}
	stats, err := analyze(ctx, client, files, opt, stderr)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 2
	}
	if err := saveReport(opt, report{Files: files, statistics: stats}); err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		if errors.Is(err, os.ErrExist) || errors.Is(err, os.ErrPermission) || errors.Is(err, os.ErrNotExist) {
			return 2
		}
		return 1
	}
	return 0
}

func analyze(ctx context.Context, client *http.Client, files []string, opt options, stderr io.Writer) (statistics, error) {
	counts := newCounter()
	err := readSources(ctx, client, files, func(line string) {
		if strings.TrimSpace(line) == "" {
			return
		}
		record, ok := parseRecord(line)
		if !ok {
			_, _ = fmt.Fprintln(stderr, "WARN: skipped malformed log line")
			return
		}
		if opt.includes(record.Time) {
			counts.add(record)
		}
	})
	if err != nil {
		return statistics{}, err
	}
	return counts.result(), nil
}

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Stderr))
}
