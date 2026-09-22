package main

import (
	"bufio"
	"fmt"
	"os"
	"sort"
)

func main() {
	if len(os.Args) != 4 {
		fmt.Fprintf(os.Stderr, "usage: %s routes.txt METHOD /path\n", os.Args[0])
		os.Exit(2)
	}

	routesPath, method, path := os.Args[1], os.Args[2], os.Args[3]

	patterns, err := loadRoutes(routesPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	type candidate struct {
		pattern *Pattern
		params  map[string]string
	}

	var matches []candidate
	for _, p := range patterns {
		if ok, params := p.Match(method, path); ok {
			matches = append(matches, candidate{p, params})
		}
	}

	if len(matches) == 0 {
		fmt.Printf("no route matches %s %s\n", method, path)
		os.Exit(1)
	}

	sort.SliceStable(matches, func(i, j int) bool {
		return matches[i].pattern.specificity() > matches[j].pattern.specificity()
	})

	winner := matches[0]
	fmt.Printf("match: %s (line %d)\n", winner.pattern.Raw, winner.pattern.Line)
	if len(winner.params) > 0 {
		keys := make([]string, 0, len(winner.params))
		for k := range winner.params {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Printf("  %s = %s\n", k, winner.params[k])
		}
	}

	if len(matches) > 1 {
		fmt.Printf("\n%d other pattern(s) also match this request:\n", len(matches)-1)
		for _, m := range matches[1:] {
			fmt.Printf("  %s (line %d)\n", m.pattern.Raw, m.pattern.Line)
		}
	}
}

func loadRoutes(path string) ([]*Pattern, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var patterns []*Pattern
	scanner := bufio.NewScanner(f)
	line := 0
	for scanner.Scan() {
		line++
		p, err := parsePattern(scanner.Text(), line)
		if err != nil {
			return nil, err
		}
		if p != nil {
			patterns = append(patterns, p)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return patterns, nil
}
