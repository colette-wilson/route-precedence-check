package main

import (
	"fmt"
	"net"
	"strings"
)

type segKind int

const (
	segLiteral segKind = iota
	segParam
	segWildcard // trailing {name...}, only valid as the last segment
)

type segment struct {
	kind  segKind
	value string // literal text, or the param/wildcard name
}

// Pattern is one line from a routes file, e.g. "GET /users/{id}/posts/".
type Pattern struct {
	Raw      string
	Line     int
	Method   string // empty means "matches any method"
	Host     string // lowercased; empty means "matches any host"
	segments []segment
	subtree  bool // pattern ends in "/", so it also matches anything below it
	exactEnd bool // pattern ends in "{$}", so it matches the directory path and nothing below it
}

func parsePattern(raw string, line int) (*Pattern, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.HasPrefix(raw, "#") {
		return nil, nil
	}

	method := ""
	rest := raw
	if sp := strings.IndexByte(raw, ' '); sp != -1 {
		candidate := raw[:sp]
		if isMethod(candidate) {
			method = candidate
			rest = strings.TrimSpace(raw[sp+1:])
		}
	}

	// Anything before the first slash is a host, as in ServeMux patterns
	// like "example.com/docs/".
	host := ""
	if !strings.HasPrefix(rest, "/") {
		slash := strings.IndexByte(rest, '/')
		if slash == -1 {
			return nil, fmt.Errorf("line %d: path must start with /: %q", line, raw)
		}
		host = rest[:slash]
		if strings.ContainsAny(host, "{} \t") {
			return nil, fmt.Errorf("line %d: invalid host %q", line, host)
		}
		host = strings.ToLower(host)
		rest = rest[slash:]
	}

	subtree := rest == "/" || strings.HasSuffix(rest, "/")
	trimmed := strings.Trim(rest, "/")

	var segs []segment
	var exactEnd bool
	if trimmed != "" {
		parts := strings.Split(trimmed, "/")
		for i, part := range parts {
			switch {
			case part == "{$}":
				if i != len(parts)-1 {
					return nil, fmt.Errorf("line %d: %q must be the last segment", line, part)
				}
				exactEnd = true
				subtree = false // {$} pins the match to exactly this path, not anything below it
			case strings.Contains(part, "{$}"):
				return nil, fmt.Errorf("line %d: %q must be its own segment", line, part)
			case strings.HasPrefix(part, "{") && strings.HasSuffix(part, "...}"):
				if i != len(parts)-1 {
					return nil, fmt.Errorf("line %d: wildcard %q must be the last segment", line, part)
				}
				segs = append(segs, segment{kind: segWildcard, value: part[1 : len(part)-4]})
				subtree = false // the wildcard already accounts for everything after it
			case strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}"):
				segs = append(segs, segment{kind: segParam, value: part[1 : len(part)-1]})
			default:
				segs = append(segs, segment{kind: segLiteral, value: part})
			}
		}
	}

	return &Pattern{Raw: raw, Line: line, Method: method, Host: host, segments: segs, subtree: subtree, exactEnd: exactEnd}, nil
}

// normalizeHost lowercases a request host and drops any port, since patterns
// name hosts without one.
func normalizeHost(host string) string {
	host = strings.ToLower(host)
	if h, _, err := net.SplitHostPort(host); err == nil {
		return h
	}
	return host
}

func isMethod(s string) bool {
	switch s {
	case "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS":
		return true
	}
	return false
}

// Match reports whether the pattern matches the given method, host and path,
// and if so, the path parameters it captured. An empty host only matches
// patterns that don't name one.
func (p *Pattern) Match(method, host, path string) (bool, map[string]string) {
	if p.Method != "" && p.Method != method {
		return false, nil
	}
	if p.Host != "" && p.Host != normalizeHost(host) {
		return false, nil
	}

	trimmed := strings.Trim(path, "/")
	var parts []string
	if trimmed != "" {
		parts = strings.Split(trimmed, "/")
	}

	params := map[string]string{}
	for i, seg := range p.segments {
		if seg.kind == segWildcard {
			if i > len(parts) {
				return false, nil
			}
			params[seg.value] = strings.Join(parts[i:], "/")
			return true, params
		}
		if i >= len(parts) {
			return false, nil
		}
		switch seg.kind {
		case segLiteral:
			if parts[i] != seg.value {
				return false, nil
			}
		case segParam:
			params[seg.value] = parts[i]
		}
	}

	if len(parts) == len(p.segments) {
		return true, params
	}
	if p.subtree && len(parts) > len(p.segments) {
		return true, params
	}
	return false, nil
}

// specificity ranks candidate matches so the "winner" can be picked when
// more than one pattern matches the same request. It follows the same
// intuition as Go 1.22's http.ServeMux (literal segments beat wildcards,
// a method-specific pattern beats a wildcard-method one) without claiming
// to reproduce every edge case of that algorithm.
func (p *Pattern) specificity() int {
	score := 0
	// A host-specific pattern always beats a hostless one, as in ServeMux,
	// so this outweighs anything the path can contribute.
	if p.Host != "" {
		score += 100000
	}
	if p.Method != "" {
		score += 1000
	}
	for _, seg := range p.segments {
		switch seg.kind {
		case segLiteral:
			score += 10
		case segParam:
			score += 3
		case segWildcard:
			score += 1
		}
	}
	if p.subtree {
		score -= 2
	}
	return score
}
