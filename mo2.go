package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// rootBuilderExclusions lets Root Builder deploy Stewie's DLL from Data while
// still leaving the base game's data files alone.
const rootBuilderExclusions = `"Saves,Morrowind.ini,Data Files,Data/*.bsa,Data/*.esm,Data/*.nam,Data/*.NAM,Data/*.txt,Data/Music,Data/Shaders,Data/Sound,Data/Video"`

var (
	errGameRunning = errors.New("close the game and Mod Organizer first")

	gamePathPattern        = regexp.MustCompile(`(?m)^gamePath=@ByteArray\((.*)\)\r?$`)
	baseDirPattern         = regexp.MustCompile(`(?m)^base_directory=(.*?)\r?$`)
	selectedProfilePattern = regexp.MustCompile(`(?m)^selected_profile=@ByteArray\((.*)\)\r?$`)
)

// install is a Begin Again copy laid out as <dir>/manager and <dir>/game.
type install struct {
	manager, game string
}

func (in install) ini() string { return filepath.Join(in.manager, "ModOrganizer.ini") }

func (in install) check() error {
	for _, f := range []string{in.ini(), filepath.Join(in.game, "FalloutNV.exe")} {
		if _, err := os.Stat(f); err != nil {
			return fmt.Errorf("%s not found; set mo2 and game in [source]", f)
		}
	}
	return nil
}

// gameRunning reports whether Mod Organizer or the game is open. Wine puts
// the program's Windows path first on its command line.
func gameRunning() bool {
	cmds, _ := filepath.Glob("/proc/[0-9]*/cmdline")
	for _, c := range cmds {
		data, err := os.ReadFile(c)
		if err != nil {
			continue
		}
		prog, _, _ := strings.Cut(strings.ToLower(string(data)), "\x00")
		if strings.HasSuffix(prog, "modorganizer.exe") || strings.HasSuffix(prog, "falloutnv.exe") {
			return true
		}
	}
	return false
}

// windowsPath returns the path Wine uses for a Linux path.
func windowsPath(p string) string { return "Z:" + strings.ReplaceAll(p, "/", `\`) }

// fixPaths replaces the build machine's game and Mod Organizer folders in
// ModOrganizer.ini with this device's.
func (in install) fixPaths() (bool, error) {
	data, err := os.ReadFile(in.ini())
	if err != nil {
		return false, err
	}
	text := string(data)

	moves := map[string]string{}
	if m := gamePathPattern.FindStringSubmatch(text); m != nil {
		moves[strings.ReplaceAll(m[1], `\\`, `\`)] = windowsPath(in.game)
	}
	if m := baseDirPattern.FindStringSubmatch(text); m != nil {
		moves[strings.ReplaceAll(m[1], "/", `\`)] = windowsPath(in.manager)
	}

	// Paths appear with escaped backslashes, forward slashes and plain backslashes.
	forms := []func(string) string{
		func(s string) string { return strings.ReplaceAll(s, `\`, `\\`) },
		func(s string) string { return strings.ReplaceAll(s, `\`, "/") },
		func(s string) string { return s },
	}
	changed := text
	for from, to := range moves {
		if from == "" || strings.EqualFold(from, to) {
			continue
		}
		for _, form := range forms {
			re := regexp.MustCompile(`(?i)` + regexp.QuoteMeta(form(from)))
			changed = re.ReplaceAllLiteralString(changed, form(to))
		}
	}
	changed = editINI(changed, []setting{
		{"General", "gamePath", "@ByteArray(" + forms[0](windowsPath(in.game)) + ")"},
		{"Settings", "base_directory", forms[1](windowsPath(in.manager))},
		{"Settings", "download_directory", forms[1](windowsPath(filepath.Join(in.manager, "downloads")))},
		{"Plugins", `RootBuilder\exclusions`, rootBuilderExclusions},
	})
	if changed == text {
		return false, nil
	}
	return true, writeFile(in.ini(), []byte(changed))
}

// fixStewie has Root Builder place Stewie's Tweaks' DLL in the game folder.
// Under Wine the DLL can't find its INI through Mod Organizer's virtual file
// system otherwise.
func (in install) fixStewie() (bool, error) {
	mods, err := os.ReadDir(filepath.Join(in.manager, "mods"))
	if err != nil {
		return false, err
	}
	for _, mod := range mods {
		dir := filepath.Join(in.manager, "mods", mod.Name())
		dll, ok := findFold(dir, "NVSE/Plugins/nvse_stewie_tweaks.dll")
		if !ok {
			continue
		}
		if err := copyFile(dll, filepath.Join(dir, "Root", "Data", "NVSE", "Plugins", "nvse_stewie_tweaks.dll")); err != nil {
			return false, err
		}
		return true, os.Rename(dll, dll+".mohidden")
	}
	return false, nil
}

func (in install) selectedProfile() (string, error) {
	data, err := os.ReadFile(in.ini())
	if err != nil {
		return "", err
	}
	m := selectedProfilePattern.FindSubmatch(data)
	if m == nil {
		return "", fmt.Errorf("no profile selected in %s", in.ini())
	}
	return string(m[1]), nil
}

// setMod turns a mod on or off in a profile's modlist. New entries go to the
// top, which is the highest priority.
func setMod(modlist, mod string, on bool) error {
	data, err := os.ReadFile(modlist)
	if err != nil {
		return err
	}
	text := string(data)
	eol := "\n"
	if strings.Contains(text, "\r\n") {
		eol = "\r\n"
	}
	lines := strings.Split(strings.TrimRight(strings.ReplaceAll(text, "\r\n", "\n"), "\n"), "\n")
	entry := "-" + mod
	if on {
		entry = "+" + mod
	}

	found := false
	top := len(lines)
	for i, l := range lines {
		if l == "+"+mod || l == "-"+mod {
			lines[i], found = entry, true
		}
		if top == len(lines) && !strings.HasPrefix(l, "#") {
			top = i
		}
	}
	if !found {
		if !on {
			return nil
		}
		lines = append(lines[:top], append([]string{entry}, lines[top:]...)...)
	}
	return writeFile(modlist, []byte(strings.Join(lines, eol)+eol))
}

// winningFile returns the highest-priority enabled mod's copy of rel,
// ignoring the mod this tool writes.
func (in install) winningFile(profile, rel string) (string, error) {
	data, err := os.ReadFile(filepath.Join(in.profileDir(profile), "modlist.txt"))
	if err != nil {
		return "", err
	}
	for _, l := range strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
		mod, ok := strings.CutPrefix(l, "+")
		if !ok || mod == touchFixMod {
			continue
		}
		if path, ok := findFold(filepath.Join(in.manager, "mods", mod), rel); ok {
			return path, nil
		}
	}
	return "", fmt.Errorf("no enabled mod has %s", rel)
}

// findFold resolves rel under base ignoring case, since mods disagree on
// names like NVSE/Plugins and NVSE/plugins.
func findFold(base, rel string) (string, bool) {
	path := base
	for _, part := range strings.Split(filepath.ToSlash(rel), "/") {
		entries, err := os.ReadDir(path)
		if err != nil {
			return "", false
		}
		found := false
		for _, e := range entries {
			if strings.EqualFold(e.Name(), part) {
				path, found = filepath.Join(path, e.Name()), true
				break
			}
		}
		if !found {
			return "", false
		}
	}
	return path, true
}

func copyFile(from, to string) error {
	src, err := os.Open(from)
	if err != nil {
		return err
	}
	defer src.Close()
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		return err
	}
	dst, err := os.Create(to)
	if err != nil {
		return err
	}
	if _, err := io.Copy(dst, src); err != nil {
		dst.Close()
		return err
	}
	return dst.Close()
}
