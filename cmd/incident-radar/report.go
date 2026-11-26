package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"os"
	"strings"
	"text/template"
)

type report struct {
	Files []string `json:"files"`
	statistics
}

var cellEscapes = strings.NewReplacer(
	"\\", "&#92;", "|", "&#124;", "\r", "&#13;", "\n", "&#10;",
	"[", "&#91;", "]", "&#93;", "`", "&#96;", "*", "&#42;", "~", "&#126;",
)

func tableCell(value string) string {
	return cellEscapes.Replace(html.EscapeString(value))
}

var markdown = template.Must(template.New("report").Funcs(template.FuncMap{
	"cell":   tableCell,
	"status": http.StatusText,
}).Parse(`# Incident Radar

## Источники

| Файл или URL |
| --- |
{{range .Files}}| {{cell .}} |
{{end}}
## Общая статистика

| Показатель | Значение |
| --- | ---: |
| Всего запросов | {{.Total}} |
| Средний размер ответа, байт | {{printf "%.2f" .Size.Average}} |
| Максимум, байт | {{.Size.Max}} |
| p95, байт | {{.Size.P95}} |

## Популярные ресурсы

| Ресурс | Запросы |
| --- | ---: |
{{range .Resources}}| {{cell .Resource}} | {{.Count}} |
{{end}}
## Коды ответа

| Код | Запросы |
| --- | ---: |
{{range .Codes}}| {{.Code}}{{with status .Code}} {{.}}{{end}} | {{.Count}} |
{{end}}`))

func (r report) writeTo(w io.Writer, format string) error {
	if format == "markdown" {
		return markdown.Execute(w, r)
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

func saveReport(opt options, r report) error {
	file, err := os.OpenFile(opt.output, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600) // #nosec G304 -- путь отчёта задаёт пользователь.
	if err != nil {
		return fmt.Errorf("создание отчёта: %w", err)
	}
	writeErr := r.writeTo(file, opt.format)
	if err := errors.Join(writeErr, file.Close()); err != nil {
		_ = os.Remove(opt.output)
		return fmt.Errorf("запись отчёта: %w", err)
	}
	return nil
}
