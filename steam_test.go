package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func shortcutsFixture() []byte {
	doc := &vdfNode{kind: vdfMap, nodes: []*vdfNode{{kind: vdfMap, key: "shortcuts", nodes: []*vdfNode{{
		kind: vdfMap, key: "0", nodes: []*vdfNode{
			{kind: vdfInt, key: "appid", num: 0x81234567},
			{kind: vdfString, key: "AppName", str: "RetroArch"},
			{kind: vdfString, key: "Exe", str: `"/usr/bin/flatpak"`},
			{kind: vdfUint64, key: "big", num: 1 << 40},
			{kind: vdfMap, key: "tags", nodes: []*vdfNode{{kind: vdfString, key: "0", str: "Emulators"}}},
		},
	}}}}}
	return doc.encode()
}

func TestVDFRoundTrip(t *testing.T) {
	data := shortcutsFixture()
	doc, err := parseVDF(data)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(doc.encode(), data) {
		t.Error("parse and encode changed the file")
	}
	if _, err := parseVDF(data[:len(data)-3]); err == nil {
		t.Error("truncated file parsed without error")
	}
}

// BEGIN_AGAIN_SHORTCUTS can point at a real shortcuts.vdf to check it
// survives a round trip.
func TestRealShortcutsRoundTrip(t *testing.T) {
	path := os.Getenv("BEGIN_AGAIN_SHORTCUTS")
	if path == "" {
		t.Skip("BEGIN_AGAIN_SHORTCUTS not set")
	}
	data := []byte(readTestFile(t, path))
	doc, err := parseVDF(data)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(doc.encode(), data) {
		t.Error("round trip changed the file")
	}
}

const configFixture = "\"InstallConfigStore\"\n{\n\t\"Software\"\n\t{\n\t\t\t\t\"CompatToolMapping\"\n\t\t\t\t{\n\t\t\t\t\t\"400\"\n\t\t\t\t\t{\n\t\t\t\t\t\t\"name\"\t\t\"proton_10\"\n\t\t\t\t\t\t\"config\"\t\t\"\"\n\t\t\t\t\t\t\"priority\"\t\t\"250\"\n\t\t\t\t\t}\n\t\t\t\t}\n\t}\n}\n"

func TestMapCompatTool(t *testing.T) {
	got, err := mapCompatTool(configFixture, "123")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "\t\t\t\t{\n\t\t\t\t\t\"123\"\n\t\t\t\t\t{\n\t\t\t\t\t\t\"name\"\t\t\"proton-experimental-arm64\"\n") {
		t.Errorf("mapping not added:\n%s", got)
	}
	if again, err := mapCompatTool(got, "123"); err != nil || again != got {
		t.Error("mapping the same app twice changed the file")
	}

	updated, err := mapCompatTool(configFixture, "400")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(updated, "proton_10") || strings.Count(updated, `"400"`) != 1 {
		t.Errorf("existing mapping not updated:\n%s", updated)
	}

	if _, err := mapCompatTool("\"x\"\n{\n}\n", "1"); err == nil {
		t.Error("missing CompatToolMapping accepted")
	}
}

// BEGIN_AGAIN_CONFIG and BEGIN_AGAIN_MAPPED_APP can name a real config.vdf
// and an app already on ARM64 Proton, which must be left untouched.
func TestRealConfigMapping(t *testing.T) {
	path, id := os.Getenv("BEGIN_AGAIN_CONFIG"), os.Getenv("BEGIN_AGAIN_MAPPED_APP")
	if path == "" || id == "" {
		t.Skip("BEGIN_AGAIN_CONFIG or BEGIN_AGAIN_MAPPED_APP not set")
	}
	text := readTestFile(t, path)
	if got, err := mapCompatTool(text, id); err != nil || got != text {
		t.Error("existing mapping changed")
	}
}
