// Package main implements the Incident Radar CLI application.
package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
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
	if err := readSources(ctx, client, files, func(string) {}); err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 2
	}
	return 0
}

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Stderr))
}
