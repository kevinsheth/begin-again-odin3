package main

import (
	"os"
	"strings"
)

// setting is one INI value. Sections and keys match case-insensitively.
type setting struct {
	section, key, value string
}

// applyINI sets each value in place. Every duplicate of a key in its section
// is updated, missing keys go at the end of their section and missing
// sections are appended. Line endings and spacing around "=" are kept.
func applyINI(path string, settings []setting) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(editINI(string(data), settings)), 0o644)
}

func editINI(text string, settings []setting) string {
	eol := "\n"
	if strings.Contains(text, "\r\n") {
		eol = "\r\n"
	}
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	type id struct{ section, key string }
	want := map[id]string{}
	for _, s := range settings {
		want[id{strings.ToLower(s.section), strings.ToLower(s.key)}] = s.value
	}

	done := map[id]bool{}
	last := map[string]int{}
	section := ""
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && strings.Contains(trimmed, "]") {
			section = strings.ToLower(strings.TrimSpace(trimmed[1:strings.Index(trimmed, "]")]))
			last[section] = i
			continue
		}
		eq := strings.Index(line, "=")
		if eq < 0 || strings.HasPrefix(trimmed, ";") || strings.HasPrefix(trimmed, "#") {
			continue
		}
		last[section] = i
		k := id{section, strings.ToLower(strings.TrimSpace(line[:eq]))}
		value, ok := want[k]
		if !ok {
			continue
		}
		rest := line[eq+1:]
		space := rest[:len(rest)-len(strings.TrimLeft(rest, " \t"))]
		lines[i] = line[:eq+1] + space + value
		done[k] = true
	}

	after := map[int][]string{}
	var tail []string
	added := map[string]bool{}
	for _, s := range settings {
		k := id{strings.ToLower(s.section), strings.ToLower(s.key)}
		if done[k] {
			continue
		}
		done[k] = true
		entry := s.key + "=" + s.value
		if i, ok := last[k.section]; ok {
			after[i] = append(after[i], entry)
			continue
		}
		if !added[k.section] {
			added[k.section] = true
			tail = append(tail, "", "["+s.section+"]")
		}
		tail = append(tail, entry)
	}

	var out []string
	for i, line := range lines {
		out = append(out, line)
		out = append(out, after[i]...)
	}
	out = append(out, tail...)
	return strings.Join(out, eol) + eol
}
