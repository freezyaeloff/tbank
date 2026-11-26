package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

type options struct {
	path, format, output string
	from, to             *time.Time
}

func parseOptions(args []string) (options, error) {
	var opt options
	flags := flag.NewFlagSet("incident-radar", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&opt.path, "path", "", "файл, glob или URL")
	flags.StringVar(&opt.path, "p", "", "сокращение для --path")
	flags.StringVar(&opt.format, "format", "json", "json или markdown")
	flags.StringVar(&opt.format, "f", "json", "сокращение для --format")
	flags.StringVar(&opt.output, "output", "", "новый файл отчёта")
	flags.StringVar(&opt.output, "o", "", "сокращение для --output")
	flags.Func("from", "начало периода YYYY-MM-DD", dateFlag(&opt.from))
	flags.Func("to", "конец периода YYYY-MM-DD", dateFlag(&opt.to))
	if err := flags.Parse(args); err != nil {
		return opt, err
	}
	if flags.NArg() != 0 {
		return opt, fmt.Errorf("лишний аргумент %q", flags.Arg(0))
	}
	if opt.path == "" || opt.output == "" {
		return opt, errors.New("укажите --path и --output")
	}
	ext := map[string]string{"json": ".json", "markdown": ".md"}[opt.format]
	if ext == "" {
		return opt, fmt.Errorf("неизвестный формат %q", opt.format)
	}
	if filepath.Ext(opt.output) != ext {
		return opt, fmt.Errorf("для формата %s расширение отчёта должно быть %s", opt.format, ext)
	}
	if opt.from != nil && opt.to != nil && !opt.from.Before(*opt.to) {
		return opt, errors.New("--from должна быть раньше --to")
	}
	if _, err := os.Lstat(opt.output); !errors.Is(err, os.ErrNotExist) {
		if err == nil {
			return opt, fmt.Errorf("файл отчёта уже существует: %s", opt.output)
		}
		return opt, fmt.Errorf("проверка файла отчёта: %w", err)
	}
	parent, err := os.Stat(filepath.Dir(opt.output))
	if err != nil {
		return opt, fmt.Errorf("каталог отчёта: %w", err)
	}
	if !parent.IsDir() {
		return opt, errors.New("родительский путь отчёта должен быть каталогом")
	}
	return opt, nil
}

func dateFlag(dst **time.Time) func(string) error {
	return func(value string) error {
		date, err := time.Parse(time.DateOnly, value)
		if err == nil {
			*dst = &date
		}
		return err
	}
}
