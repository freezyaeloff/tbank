package main

import (
	"cmp"
	"math"
	"slices"
)

type resourceCount struct {
	Resource string
	Count    int
}

type codeCount struct {
	Code  int
	Count int
}

type sizeStats struct {
	Average  float64
	Max, P95 int64
}

type statistics struct {
	Total     int
	Size      sizeStats
	Resources []resourceCount
	Codes     []codeCount
}

type counter struct {
	resources map[string]int
	codes     map[int]int
	sizes     []int64
	sum       float64
}

func newCounter() *counter {
	return &counter{resources: make(map[string]int), codes: make(map[int]int)}
}

func (c *counter) add(rec LogRecord) {
	c.resources[rec.Resource]++
	c.codes[rec.Status]++
	c.sizes = append(c.sizes, rec.Size)
	c.sum += float64(rec.Size)
}

func (c *counter) result() statistics {
	n := len(c.sizes)
	s := statistics{
		Total:     n,
		Resources: make([]resourceCount, 0, len(c.resources)),
		Codes:     make([]codeCount, 0, len(c.codes)),
	}
	for resource, count := range c.resources {
		s.Resources = append(s.Resources, resourceCount{resource, count})
	}
	slices.SortFunc(s.Resources, func(a, b resourceCount) int {
		if a.Count != b.Count {
			return cmp.Compare(b.Count, a.Count)
		}
		return cmp.Compare(a.Resource, b.Resource)
	})
	s.Resources = s.Resources[:min(10, len(s.Resources))]
	for code, count := range c.codes {
		s.Codes = append(s.Codes, codeCount{code, count})
	}
	slices.SortFunc(s.Codes, func(a, b codeCount) int { return cmp.Compare(a.Code, b.Code) })
	if n > 0 {
		slices.Sort(c.sizes)
		s.Size = sizeStats{
			Average: math.Round(c.sum/float64(n)*100) / 100,
			Max:     c.sizes[n-1],
			P95:     c.sizes[n-n/20-1], // ceil(0.95*n) = n - floor(n/20).
		}
	}
	return s
}
