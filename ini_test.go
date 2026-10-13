package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTestFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func crlf(lines ...string) string { return strings.Join(lines, "\r\n") + "\r\n" }

func TestEditINI(t *testing.T) {
	tests := []struct {
		name     string
		in       string
		settings []setting
		want     string
	}{
		{
			name: "updates in place and keeps CRLF and spacing",
			in:   crlf("[Display]", "iSize W = 1920"),
			settings: []setting{
				{"display", "isize w", "1600"},
			},
			want: crlf("[Display]", "iSize W = 1600"),
		},
		{
			name:     "updates every duplicate and skips comments",
			in:       crlf("[Grass]", "b30GrassVS=1", "; b30GrassVS=1", "b30GrassVS=1"),
			settings: []setting{{"Grass", "b30GrassVS", "0"}},
			want:     crlf("[Grass]", "b30GrassVS=0", "; b30GrassVS=1", "b30GrassVS=0"),
		},
		{
			name:     "adds a missing key to its section",
			in:       "[A]\nx=1\n\n[B]\ny=2\n",
			settings: []setting{{"A", "z", "3"}},
			want:     "[A]\nx=1\nz=3\n\n[B]\ny=2\n",
		},
		{
			name:     "appends a missing section",
			in:       "[A]\nx=1\n",
			settings: []setting{{"LOD", "fLODFadeOutMultItems", "3"}},
			want:     "[A]\nx=1\n\n[LOD]\nfLODFadeOutMultItems=3\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := editINI(tt.in, tt.settings)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
			if again := editINI(got, tt.settings); again != got {
				t.Errorf("second run changed the file to %q", again)
			}
		})
	}
}

func TestParseINI(t *testing.T) {
	got := parseINI("; comment\n[Display]\r\n iSize W = 1600 \r\n\n[LOD]\nx=\n")
	want := []setting{{"Display", "iSize W", "1600"}, {"LOD", "x", ""}}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("setting %d: got %v, want %v", i, got[i], want[i])
		}
	}
}

func TestParseConfig(t *testing.T) {
	c, err := parseConfig(configTemplate + "\n[source]\nmo2 = nas:/mo2\ngame = /mnt/game\n[Display]\niSize W=1280\n")
	if err != nil {
		t.Fatal(err)
	}
	home, _ := os.UserHomeDir()
	if c.mo2 != "nas:/mo2" || c.game != "/mnt/game" || c.dir != filepath.Join(home, "Games", "BeginAgain") || !c.touchscreenFix || c.verify {
		t.Errorf("unexpected config %+v", c)
	}
	if len(c.gameINI) != 1 || c.gameINI[0] != (setting{"Display", "iSize W", "1280"}) {
		t.Errorf("game settings %v", c.gameINI)
	}
	if _, err := parseConfig("[source]\ntypo = 1\n"); err == nil {
		t.Error("unknown setting accepted")
	}
}
