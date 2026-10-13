package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseSource(t *testing.T) {
	tests := map[string]source{
		"nas:/volume1/Begin Again":   {"nas", "/volume1/Begin Again"},
		"/run/media/usb/Begin:Again": {"", "/run/media/usb/Begin:Again"},
		"./mo2":                      {"", "./mo2"},
	}
	for in, want := range tests {
		if got := parseSource(in); got != want {
			t.Errorf("parseSource(%q) = %+v, want %+v", in, got, want)
		}
	}
}

func TestFetch(t *testing.T) {
	src := t.TempDir()
	writeTestFile(t, filepath.Join(src, "mods", "a", "big.esp"), "contents")
	writeTestFile(t, filepath.Join(src, "ModOrganizer.exe"), "exe")
	writeTestFile(t, filepath.Join(src, "profiles", "custom.ini"), "")
	writeTestFile(t, filepath.Join(src, "downloads", "archive.7z"), "skip me")
	dest := filepath.Join(t.TempDir(), "manager")
	writeTestFile(t, filepath.Join(dest, "mods", "a", "big.esp"), "cont")

	if err := fetchOnce(src, dest, []string{"downloads"}, true); err != nil {
		t.Fatal(err)
	}
	if got := readTestFile(t, filepath.Join(dest, "mods", "a", "big.esp")); got != "contents" {
		t.Errorf("partial file is %q", got)
	}
	if _, err := os.Stat(filepath.Join(dest, "profiles", "custom.ini")); err != nil {
		t.Error("empty file not copied")
	}
	if _, err := os.Stat(filepath.Join(dest, "downloads")); err == nil {
		t.Error("downloads copied")
	}

	writeTestFile(t, filepath.Join(dest, "ModOrganizer.exe"), "edited")
	if err := fetchOnce(src, dest, []string{"downloads"}, false); err != nil {
		t.Fatal(err)
	}
	if got := readTestFile(t, filepath.Join(dest, "ModOrganizer.exe")); got != "edited" {
		t.Error("a finished copy was redone")
	}
	if err := compareHashes(source{path: src}, dest, map[string]int64{"ModOrganizer.exe": 3}); err == nil {
		t.Error("changed file passed verification")
	}
}

// BEGIN_AGAIN_REMOTE can name a small host:path folder to test copying over
// SSH.
func TestRemoteFetch(t *testing.T) {
	remote := os.Getenv("BEGIN_AGAIN_REMOTE")
	if remote == "" {
		t.Skip("BEGIN_AGAIN_REMOTE not set")
	}
	if err := fetch(parseSource(remote), t.TempDir(), nil, true); err != nil {
		t.Fatal(err)
	}
}
