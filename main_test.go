package main

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"
	"testing"
)

type parsedLine struct {
	kind  string
	order int
	zone  int
}

func parseOutput(t *testing.T, out string) []parsedLine {
	t.Helper()
	var lines []parsedLine
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		var l parsedLine
		var rest int
		text := sc.Text()
		switch {
		case strings.HasPrefix(text, "CREATED"):
			fmt.Sscanf(text, "CREATED order=%d restaurant=%d type=%d", &l.order, &rest, &l.zone)
			l.kind = "CREATED"
		case strings.HasPrefix(text, "DONE"):
			fmt.Sscanf(text, "DONE total=%d", &l.order)
			l.kind = "DONE"
		default:
			var kind string
			if _, err := fmt.Sscanf(text, "%s order=%d zone=%d", &kind, &l.order, &l.zone); err != nil {
				t.Fatalf("unparseable line %q: %v", text, err)
			}
			l.kind = kind
		}
		lines = append(lines, l)
	}
	return lines
}

// TestPipeline checks that every order goes through all four stages exactly
// once, in order, and that no zone ever runs more orders than it has tokens.
func TestPipeline(t *testing.T) {
	cases := []Config{
		{N: 50, Restaurants: 3, Zones: 3, SeedA: 1, SeedB: 2, Tokens: []int{1, 2, 3}},
		{N: 40, Restaurants: 4, Zones: 2, SeedA: 7, SeedB: 9, Tokens: []int{1, 1}},
		{N: 7, Restaurants: 10, Zones: 1, SeedA: 3, SeedB: 4, Tokens: []int{2}}, // more restaurants than orders
		{N: 0, Restaurants: 2, Zones: 2, Tokens: []int{1, 1}},
	}

	for i, cfg := range cases {
		t.Run(fmt.Sprintf("case%d", i), func(t *testing.T) {
			var buf bytes.Buffer
			run(cfg, &buf)
			lines := parseOutput(t, buf.String())

			stage := map[string]int{"CREATED": 1, "DISPATCHED": 2, "STARTED": 3, "COMPLETED": 4}
			progress := make(map[int]int)  // order -> last stage seen
			orderZone := make(map[int]int) // order -> zone from CREATED
			running := make([]int, cfg.Zones)

			for idx, l := range lines {
				if l.kind == "DONE" {
					if idx != len(lines)-1 {
						t.Fatalf("DONE is not the last line")
					}
					if l.order != cfg.N {
						t.Fatalf("DONE total=%d, want %d", l.order, cfg.N)
					}
					continue
				}
				s, ok := stage[l.kind]
				if !ok {
					t.Fatalf("unexpected event %q", l.kind)
				}
				if progress[l.order] != s-1 {
					t.Fatalf("order %d: %s after stage %d", l.order, l.kind, progress[l.order])
				}
				progress[l.order] = s

				if l.kind == "CREATED" {
					orderZone[l.order] = l.zone
				} else if l.zone != orderZone[l.order] {
					t.Fatalf("order %d went to zone %d, created for zone %d", l.order, l.zone, orderZone[l.order])
				}

				switch l.kind {
				case "STARTED":
					running[l.zone]++
					if running[l.zone] > cfg.Tokens[l.zone] {
						t.Fatalf("zone %d running %d orders, limit %d", l.zone, running[l.zone], cfg.Tokens[l.zone])
					}
				case "COMPLETED":
					running[l.zone]--
				}
			}

			if len(progress) != cfg.N {
				t.Fatalf("saw %d orders, want %d", len(progress), cfg.N)
			}
			for id, s := range progress {
				if s != 4 {
					t.Fatalf("order %d stopped at stage %d", id, s)
				}
			}
		})
	}
}

// TestDeterministicRouting checks that the same seed routes orders the same way.
func TestDeterministicRouting(t *testing.T) {
	cfg := Config{N: 30, Restaurants: 3, Zones: 4, SeedA: 42, SeedB: 1, Tokens: []int{2, 2, 2, 2}}
	routing := func() map[int]int {
		var buf bytes.Buffer
		run(cfg, &buf)
		m := make(map[int]int)
		for _, l := range parseOutput(t, buf.String()) {
			if l.kind == "CREATED" {
				m[l.order] = l.zone
			}
		}
		return m
	}
	a, b := routing(), routing()
	for id, z := range a {
		if b[id] != z {
			t.Fatalf("order %d routed to zone %d then %d", id, z, b[id])
		}
	}
}

func TestValidate(t *testing.T) {
	bad := []Config{
		{N: 5, Restaurants: 1, Zones: 3, Tokens: []int{1}},    // token count mismatch
		{N: 5, Restaurants: 1, Zones: 0, Tokens: nil},         // no zones
		{N: 5, Restaurants: 0, Zones: 1, Tokens: []int{1}},    // no restaurants
		{N: 5, Restaurants: 1, Zones: 2, Tokens: []int{1, 0}}, // zero tokens would deadlock
		{N: -1, Restaurants: 1, Zones: 1, Tokens: []int{1}},
	}
	for _, cfg := range bad {
		if cfg.Validate() == nil {
			t.Errorf("expected error for %+v", cfg)
		}
	}
	good := Config{N: 5, Restaurants: 1, Zones: 2, Tokens: []int{1, 3}}
	if err := good.Validate(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
