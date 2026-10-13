package main

import (
	"os"
	"path/filepath"
	"strings"
)

// setting is one INI value. Sections and keys match case-insensitively.
type setting struct {
	section, key, value string
}

// parseINI returns every key in text in order. Comments and blank lines are
// skipped.
func parseINI(text string) []setting {
	var settings []setting
	section := ""
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case line == "", strings.HasPrefix(line, ";"), strings.HasPrefix(line, "#"):
		case strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]"):
			section = strings.TrimSpace(line[1 : len(line)-1])
		default:
			if key, value, ok := strings.Cut(line, "="); ok {
				settings = append(settings, setting{section, strings.TrimSpace(key), strings.TrimSpace(value)})
			}
		}
	}
	return settings
}

// applyINI sets each value in place. Every duplicate of a key in its section
// is updated, missing keys go at the end of their section and missing
// sections are appended. Line endings and spacing around "=" are kept.
func applyINI(path string, settings []setting) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return writeFile(path, []byte(editINI(string(data), settings)))
}

// writeFile replaces path atomically so an interrupted run can't leave a
// half-written file behind.
func writeFile(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
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
