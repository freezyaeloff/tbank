package main

import (
	"fmt"
	"slices"
	"testing"
)

func TestCountsAndTopTen(t *testing.T) {
	c := newCounter()
	for _, status := range []int{200, 200, 404, 500, 500, 500} {
		c.add(LogRecord{Resource: "/popular", Status: status})
	}
	s := c.result()
	if s.Total != 6 || !slices.Equal(s.Codes, []codeCount{{200, 2}, {404, 1}, {500, 3}}) {
		t.Fatalf("unexpected distribution: %+v", s)
	}
	for i := 11; i >= 0; i-- {
		c.add(LogRecord{Resource: fmt.Sprintf("/%02d", i), Status: 200})
	}
	top := c.result().Resources
	if len(top) != 10 || top[0] != (resourceCount{"/popular", 6}) {
		t.Fatalf("unexpected top ten: %v", top)
	}
	for i := 1; i < len(top); i++ {
		if want := (resourceCount{fmt.Sprintf("/%02d", i-1), 1}); top[i] != want {
			t.Errorf("rank %d = %v, want %v", i+1, top[i], want)
		}
	}
}

func TestResponseSizes(t *testing.T) {
	for _, tc := range []struct {
		name  string
		sizes []int64
		want  sizeStats
	}{
		{"empty", nil, sizeStats{}},
		{"example", []int64{400, 100, 500, 200, 300}, sizeStats{300, 500, 500}},
		{"rounding", []int64{1, 2, 2}, sizeStats{1.67, 2, 2}},
		{"p95 below max", append(make([]int64, 19), 100), sizeStats{5, 100, 0}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := newCounter()
			for _, size := range tc.sizes {
				c.add(LogRecord{Size: size})
			}
			if got := c.result().Size; got != tc.want {
				t.Fatalf("size = %+v, want %+v", got, tc.want)
			}
		})
	}
}
