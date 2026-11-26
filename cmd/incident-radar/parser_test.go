package main

import (
	"bytes"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const exampleLine = `93.180.71.3 - - [17/May/2015:08:05:32 +0300] "GET /downloads/product_1 HTTP/1.1" 304 0 "-" "Debian APT-HTTP/1.3"`

func TestParseRecord(t *testing.T) {
	rec, ok := parseRecord(exampleLine)
	if !ok || rec.IP.String() != "93.180.71.3" || rec.Time.Format(time.RFC3339) != "2015-05-17T08:05:32+03:00" ||
		rec.Method != "GET" || rec.Resource != "/downloads/product_1" || rec.Protocol != "HTTP/1.1" ||
		rec.Status != 304 || rec.Size != 0 {
		t.Fatalf("record = %+v, ok = %v", rec, ok)
	}
	for _, tc := range []struct{ name, old, invalid string }{
		{"address", "93.180.71.3", "999.0.0.1"},
		{"date", "17/May", "32/May"},
		{"request", " HTTP/1.1", ""},
		{"status", "304", "err"},
		{"size", "304 0", "304 -1"},
		{"truncated", `"Debian APT-HTTP/1.3"`, `"unterminated`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, valid := parseRecord(strings.Replace(exampleLine, tc.old, tc.invalid, 1)); valid {
				t.Fatal("accepted malformed line")
			}
		})
	}
}

func TestDateFilter(t *testing.T) {
	from := time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 0, 1)
	dates := []string{
		"2025-02-28T23:59:59-07:00", "2025-03-01T00:00:00+07:00",
		"2025-03-02T23:59:59-07:00", "2025-03-03T00:00:00+07:00",
	}
	for _, tc := range []struct {
		name string
		opt  options
		want [4]bool
	}{
		{"all", options{}, [4]bool{true, true, true, true}},
		{"from", options{from: &from}, [4]bool{false, true, true, true}},
		{"to", options{to: &to}, [4]bool{true, true, true, false}},
		{"both", options{from: &from, to: &to}, [4]bool{false, true, true, false}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for i, value := range dates {
				stamp, err := time.Parse(time.RFC3339, value)
				if err != nil {
					t.Fatal(err)
				}
				if got := tc.opt.includes(stamp); got != tc.want[i] {
					t.Errorf("includes(%s) = %v, want %v", value, got, tc.want[i])
				}
			}
		})
	}
}

func TestMalformedLineDoesNotStopProcessing(t *testing.T) {
	file := filepath.Join(t.TempDir(), "mixed.log")
	putFile(t, file, strings.Join([]string{exampleLine, "", "wrong", exampleLine}, "\n"))
	var warnings bytes.Buffer
	s, err := analyze(t.Context(), http.DefaultClient, []string{file}, options{}, &warnings)
	if err != nil || s.Total != 2 || warnings.String() != "WARN: skipped malformed log line\n" {
		t.Fatalf("total = %d, warnings = %q, err = %v", s.Total, warnings.String(), err)
	}
}
