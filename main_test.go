package main

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func write(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func crlf(lines ...string) string { return strings.Join(lines, "\r\n") + "\r\n" }

func TestEditINI(t *testing.T) {
	in := crlf("[Display]", "iSize W = 1920", "[grass]", "b30GrassVS=1", "; b30GrassVS=1", "b30GrassVS=1", "", "[General]", "sLanguage=ENGLISH")
	got := editINI(in, []setting{
		{"display", "isize w", "1600"},
		{"Display", "iSize H", "900"},
		{"Grass", "b30GrassVS", "0"},
		{"LOD", "fLODFadeOutMultItems", "3"},
	})
	want := crlf("[Display]", "iSize W = 1600", "iSize H=900", "[grass]", "b30GrassVS=0", "; b30GrassVS=1", "b30GrassVS=0", "", "[General]", "sLanguage=ENGLISH", "", "[LOD]", "fLODFadeOutMultItems=3")
	if got != want {
		t.Errorf("got\n%q\nwant\n%q", got, want)
	}
	if again := editINI(got, []setting{{"Display", "iSize W", "1600"}}); again != got {
		t.Error("applying the same value twice changed the file")
	}
}

func TestEditINIKeepsLF(t *testing.T) {
	got := editINI("[A]\nx=1\n", []setting{{"A", "x", "2"}})
	if got != "[A]\nx=2\n" {
		t.Errorf("got %q", got)
	}
}

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

// BEGIN_AGAIN_SHORTCUTS can point at a real shortcuts.vdf to check it survives.
func TestRealShortcutsRoundTrip(t *testing.T) {
	path := os.Getenv("BEGIN_AGAIN_SHORTCUTS")
	if path == "" {
		t.Skip("BEGIN_AGAIN_SHORTCUTS not set")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := parseVDF(data)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(doc.encode(), data) {
		t.Error("real shortcuts.vdf changed on round trip")
	}
}

// BEGIN_AGAIN_CONFIG can point at a real config.vdf. Remapping an app that
// already uses ARM64 Proton must leave it untouched.
func TestRealConfigMapping(t *testing.T) {
	path, id := os.Getenv("BEGIN_AGAIN_CONFIG"), os.Getenv("BEGIN_AGAIN_MAPPED_APP")
	if path == "" || id == "" {
		t.Skip("BEGIN_AGAIN_CONFIG or BEGIN_AGAIN_MAPPED_APP not set")
	}
	text := read(t, path)
	got, err := mapCompatTool(text, id)
	if err != nil || got != text {
		t.Error("existing ARM64 mapping changed")
	}
	added, err := mapCompatTool(text, "123")
	if err != nil || len(added) <= len(text) {
		t.Error("new mapping not added")
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
	again, err := mapCompatTool(got, "123")
	if err != nil || again != got {
		t.Error("mapping the same app twice changed the file")
	}
	moved, _ := mapCompatTool(configFixture, "400")
	if strings.Contains(moved, "proton_10") || strings.Count(moved, "\"400\"") != 1 {
		t.Errorf("existing mapping not updated:\n%s", moved)
	}
	if _, err := mapCompatTool("\"x\"\n{\n}\n", "1"); err == nil {
		t.Error("missing CompatToolMapping didn't error")
	}
}

func dsoalZipFixture(t *testing.T) []byte {
	t.Helper()
	pack := func(files map[string]string) []byte {
		var buf bytes.Buffer
		w := zip.NewWriter(&buf)
		for name, body := range files {
			f, _ := w.Create(name)
			f.Write([]byte(body))
		}
		w.Close()
		return buf.Bytes()
	}
	inner := pack(map[string]string{
		"DSOAL/Win32/dsound.dll":                "dsound32",
		"DSOAL/Win32/dsoal-aldrv.dll":           "aldrv32",
		"DSOAL/Win64/dsound.dll":                "dsound64",
		"DSOAL/Documentation/DSOAL-License.txt": "license",
		"DSOAL/Documentation/DSOAL-ReadMe.txt":  "readme",
		"DSOAL+HRTF/Win32/dsound.dll":           "hrtf",
	})
	return pack(map[string]string{"DSOAL_r695.zip": string(inner)})
}

// fakeInstall lays out a Begin Again copy as it arrives from a Windows PC.
func fakeInstall(t *testing.T) (dir, steam string) {
	dir = t.TempDir()
	m := filepath.Join(dir, "manager")
	write(t, filepath.Join(m, "ModOrganizer.ini"), crlf(
		"[General]",
		`gamePath=@ByteArray(D:\\SteamLibrary\\steamapps\\common\\Fallout New Vegas)`,
		"selected_profile=@ByteArray(Begin Again)",
		"[customExecutables]",
		"1\\binary=D:/SteamLibrary/steamapps/common/Fallout New Vegas/FalloutNV.exe",
		"1\\title=Begin Again",
		"1\\workingDirectory=D:/SteamLibrary/steamapps/common/fallout new vegas",
		"2\\binary=C:/Games/Begin Again/tools/FNVEdit.exe",
		"[Settings]",
		"base_directory=C:/Games/Begin Again",
		"download_directory=C:/Games/Begin Again/downloads",
		"[Plugins]",
		`RootBuilder\exclusions="Data"`,
	))
	write(t, filepath.Join(dir, "game", "FalloutNV.exe"), "exe")
	write(t, filepath.Join(dir, "game", "dxvk.conf"), "d3d9.maxFrameRate = 60\n")
	p := filepath.Join(m, "profiles", "Begin Again")
	write(t, filepath.Join(p, "modlist.txt"), crlf("# This file was automatically generated by Mod Organizer.", "+Begin Again Configs", "+lStewieAl's Tweaks and Engine Fixes"))
	write(t, filepath.Join(p, "settings.ini"), crlf("[General]", "LocalSaves=false"))
	write(t, filepath.Join(p, "falloutprefs.ini"), crlf("[Display]", `sD3DDevice="AMD Radeon 780M Graphics"`, "iSize W=1920", "iSize H=1080", "iMultiSample=2", "iMaxAnisotropy=15", "[Grass]", "b30GrassVS=1"))
	write(t, filepath.Join(p, "falloutcustom.ini"), crlf("[Display]", "iSize W=1920", "iSize H=1080"))
	configs := filepath.Join(m, "mods", "Begin Again Configs", "NVSE", "Plugins")
	write(t, filepath.Join(configs, "nvse_stewie_tweaks.ini"), crlf("[Tweaks]", "bToggleControllerIfAttackPressed=1", "sFrom=configs"))
	write(t, filepath.Join(configs, "VanillaPlusAO.ini"), crlf("[Exterior]", "bEnabled = 1", "[Interior]", "bEnabled = 1"))
	write(t, filepath.Join(m, "mods", "lStewieAl's Tweaks and Engine Fixes", "NVSE", "plugins", "nvse_stewie_tweaks.dll"), "stewie")

	steam = filepath.Join(dir, "Steam")
	write(t, filepath.Join(steam, "userdata", "12345", "config", "shortcuts.vdf"), string(shortcutsFixture()))
	write(t, filepath.Join(steam, "config", "config.vdf"), configFixture)
	return dir, steam
}

func TestSetupAndSettings(t *testing.T) {
	dir, steam := fakeInstall(t)
	zipPath := filepath.Join(dir, "dsoal.zip")
	write(t, zipPath, string(dsoalZipFixture(t)))
	m := filepath.Join(dir, "manager")
	odin := filepath.Join(m, "profiles", "Odin")

	args := []string{"--dir", dir, "--steam-dir", steam, "--dsoal-zip", zipPath}
	if err := runSetup(args); err != nil {
		t.Fatal(err)
	}

	mo := read(t, filepath.Join(m, "ModOrganizer.ini"))
	game := windowsPath(filepath.Join(dir, "game"))
	for _, want := range []string{
		"gamePath=@ByteArray(" + strings.ReplaceAll(game, `\`, `\\`) + ")\r\n",
		"1\\binary=" + strings.ReplaceAll(game, `\`, "/") + "/FalloutNV.exe\r\n",
		"1\\workingDirectory=" + strings.ReplaceAll(game, `\`, "/") + "\r\n",
		"2\\binary=" + strings.ReplaceAll(windowsPath(m), `\`, "/") + "/tools/FNVEdit.exe\r\n",
		"selected_profile=@ByteArray(Odin)\r\n",
		"RootBuilder\\exclusions=" + rootBuilderExclusions + "\r\n",
	} {
		if !strings.Contains(mo, want) {
			t.Errorf("ModOrganizer.ini lacks %q:\n%s", want, mo)
		}
	}

	stewie := filepath.Join(m, "mods", "lStewieAl's Tweaks and Engine Fixes")
	if read(t, filepath.Join(stewie, "Root", "Data", "NVSE", "Plugins", "nvse_stewie_tweaks.dll")) != "stewie" {
		t.Error("Stewie DLL not copied for Root Builder")
	}
	if _, err := os.Stat(filepath.Join(stewie, "NVSE", "plugins", "nvse_stewie_tweaks.dll.mohidden")); err != nil {
		t.Error("original Stewie DLL not hidden")
	}

	if read(t, filepath.Join(m, "mods", "DSOAL", "Root", "dsound.dll")) != "dsound32" {
		t.Error("DSOAL Win32 dsound.dll not installed")
	}
	if _, err := os.Stat(filepath.Join(m, "mods", "DSOAL", "Root", "DSOAL-licenses", "DSOAL-License.txt")); err != nil {
		t.Error("DSOAL license not kept")
	}

	prefs := read(t, filepath.Join(odin, "falloutprefs.ini"))
	for _, want := range []string{"iSize W=1600\r\n", "iMultiSample=0\r\n", `sD3DDevice="AMD Radeon RX 6700 XT"` + "\r\n", "b30GrassVS=0\r\n"} {
		if !strings.Contains(prefs, want) {
			t.Errorf("falloutprefs.ini lacks %q", want)
		}
	}
	if !strings.Contains(read(t, filepath.Join(odin, "falloutcustom.ini")), "iSize H=900\r\n") {
		t.Error("falloutcustom.ini not updated")
	}
	if !strings.Contains(read(t, filepath.Join(odin, "settings.ini")), "LocalSettings=true") {
		t.Error("profile doesn't use its own INIs")
	}
	if read(t, filepath.Join(m, "profiles", "Begin Again", "falloutprefs.ini")) != crlf("[Display]", `sD3DDevice="AMD Radeon 780M Graphics"`, "iSize W=1920", "iSize H=1080", "iMultiSample=2", "iMaxAnisotropy=15", "[Grass]", "b30GrassVS=1") {
		t.Error("source profile was changed")
	}

	modlist := read(t, filepath.Join(odin, "modlist.txt"))
	if !strings.HasPrefix(modlist, crlf("# This file was automatically generated by Mod Organizer.", "+DSOAL", "+Odin Ambient Occlusion Off", "+Odin Touchscreen Fix")) {
		t.Errorf("modlist order wrong:\n%s", modlist)
	}
	touch := read(t, filepath.Join(m, "mods", touchFixMod, "NVSE", "Plugins", "nvse_stewie_tweaks.ini"))
	if !strings.Contains(touch, "bToggleControllerIfAttackPressed=0\r\n") || !strings.Contains(touch, "sFrom=configs") {
		t.Errorf("touch fix wrong:\n%s", touch)
	}
	if !strings.Contains(read(t, filepath.Join(m, "mods", aoOffMod, "NVSE", "Plugins", "VanillaPlusAO.ini")), "bEnabled = 0\r\n") {
		t.Error("AO not turned off")
	}

	doc, err := parseVDF([]byte(read(t, filepath.Join(steam, "userdata", "12345", "config", "shortcuts.vdf"))))
	if err != nil {
		t.Fatal(err)
	}
	list := doc.child("shortcuts").nodes
	if len(list) != 2 || list[0].child("big").num != 1<<40 {
		t.Fatal("existing shortcut lost or changed")
	}
	entry := list[1]
	if entry.child("Exe").str != `"`+filepath.Join(m, "ModOrganizer.exe")+`"` || !strings.HasPrefix(entry.child("LaunchOptions").str, `WINEDLLOVERRIDES="dsound=n,b" DXVK_CONFIG_FILE=`) {
		t.Errorf("shortcut wrong: %+v", entry)
	}
	id := entry.child("appid").num
	if !strings.Contains(read(t, filepath.Join(steam, "config", "config.vdf")), "\""+strconv.FormatUint(id, 10)+"\"\n\t\t\t\t\t{\n\t\t\t\t\t\t\"name\"\t\t\"proton-experimental-arm64\"") {
		t.Error("compatibility tool not set")
	}
	if _, err := os.Stat(filepath.Join(steam, "config", "config.vdf.before-begin-again")); err != nil {
		t.Error("config.vdf not backed up")
	}

	// A second run changes nothing and doesn't duplicate the shortcut.
	if err := runSetup(args); err != nil {
		t.Fatal(err)
	}
	doc, _ = parseVDF([]byte(read(t, filepath.Join(steam, "userdata", "12345", "config", "shortcuts.vdf"))))
	if n := len(doc.child("shortcuts").nodes); n != 2 {
		t.Errorf("rerun made %d shortcuts", n)
	}

	// Changing one setting keeps the rest.
	if err := runSettings([]string{"--dir", dir, "--anisotropy", "8", "--ao", "on"}); err != nil {
		t.Fatal(err)
	}
	prefs = read(t, filepath.Join(odin, "falloutprefs.ini"))
	if !strings.Contains(prefs, "iMaxAnisotropy=8\r\n") || !strings.Contains(prefs, "iSize W=1600\r\n") {
		t.Errorf("settings change lost values:\n%s", prefs)
	}
	if !strings.Contains(read(t, filepath.Join(odin, "modlist.txt")), "-Odin Ambient Occlusion Off\r\n") {
		t.Error("AO override still on")
	}

	// Going back to defaults restores Begin Again's values.
	if err := runSettings([]string{"--dir", dir, "--preset", "quality"}); err != nil {
		t.Fatal(err)
	}
	prefs = read(t, filepath.Join(odin, "falloutprefs.ini"))
	if !strings.Contains(prefs, "iMultiSample=2\r\n") || !strings.Contains(prefs, "iMaxAnisotropy=15\r\n") || !strings.Contains(prefs, "b30GrassVS=1\r\n") {
		t.Errorf("quality didn't restore Begin Again's values:\n%s", prefs)
	}

	if err := runSettings([]string{"--dir", dir, "--msaa", "lots"}); err == nil {
		t.Error("bad value accepted")
	}
}

func TestFetchResumesAndVerifies(t *testing.T) {
	src := t.TempDir()
	write(t, filepath.Join(src, "mods", "a", "big.esp"), "contents")
	write(t, filepath.Join(src, "ModOrganizer.exe"), "exe")
	write(t, filepath.Join(src, "profiles", "custom.ini"), "")
	write(t, filepath.Join(src, "downloads", "archive.7z"), "skip me")
	dest := filepath.Join(t.TempDir(), "manager")
	write(t, filepath.Join(dest, "mods", "a", "big.esp"), "cont")

	if err := fetchOnce(src, dest, []string{"downloads"}, true); err != nil {
		t.Fatal(err)
	}
	if read(t, filepath.Join(dest, "mods", "a", "big.esp")) != "contents" {
		t.Error("partial file not recopied")
	}
	if _, err := os.Stat(filepath.Join(dest, "profiles", "custom.ini")); err != nil {
		t.Error("empty file not copied")
	}
	if _, err := os.Stat(filepath.Join(dest, "downloads")); err == nil {
		t.Error("downloads copied")
	}

	write(t, filepath.Join(dest, "ModOrganizer.exe"), "edited by setup")
	if err := fetchOnce(src, dest, []string{"downloads"}, false); err != nil {
		t.Fatal(err)
	}
	if read(t, filepath.Join(dest, "ModOrganizer.exe")) != "edited by setup" {
		t.Error("finished copy was redone")
	}

	other := filepath.Join(t.TempDir(), "game")
	write(t, filepath.Join(other, "x"), "abc")
	if err := fetch(source{path: src}, other, nil, false); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(other, "ModOrganizer.exe"), "xyz")
	if err := compareHashes(source{path: src}, other, map[string]int64{"ModOrganizer.exe": 3}); err == nil {
		t.Error("changed file passed verification")
	}
}

func TestParseSource(t *testing.T) {
	for in, want := range map[string]source{
		"nas:/volume1/Begin Again":   {"nas", "/volume1/Begin Again"},
		"/run/media/usb/Begin:Again": {"", "/run/media/usb/Begin:Again"},
		"./mo2":                      {"", "./mo2"},
	} {
		if got := parseSource(in); got != want {
			t.Errorf("%q parsed as %+v", in, got)
		}
	}
}

// BEGIN_AGAIN_REMOTE can name a small host:path folder to test copying over SSH.
func TestRemoteFetch(t *testing.T) {
	remote := os.Getenv("BEGIN_AGAIN_REMOTE")
	if remote == "" {
		t.Skip("BEGIN_AGAIN_REMOTE not set")
	}
	dest := t.TempDir()
	if err := fetch(parseSource(remote), dest, nil, true); err != nil {
		t.Fatal(err)
	}
}
