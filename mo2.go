package main

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const rootBuilderExclusions = `"Saves,Morrowind.ini,Data Files,Data/*.bsa,Data/*.esm,Data/*.nam,Data/*.NAM,Data/*.txt,Data/Music,Data/Shaders,Data/Sound,Data/Video"`

// install is a Begin Again copy laid out as <dir>/manager and <dir>/game.
type install struct {
	manager, game string
}

func (in install) ini() string { return filepath.Join(in.manager, "ModOrganizer.ini") }

func (in install) check() error {
	for _, f := range []string{in.ini(), filepath.Join(in.game, "FalloutNV.exe")} {
		if _, err := os.Stat(f); err != nil {
			return fmt.Errorf("can't find %s. Pass --mo2 and --game so setup can copy Begin Again over", f)
		}
	}
	if gameRunning() {
		return fmt.Errorf("close the game and Mod Organizer first")
	}
	return nil
}

// gameRunning reports whether Mod Organizer or the game is open. Wine puts
// the Windows path of the program first on its command line.
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

// windowsPath is how Wine sees a Linux path.
func windowsPath(p string) string { return "Z:" + strings.ReplaceAll(p, "/", `\`) }

// fixPaths swaps the PC's game and Mod Organizer folders in ModOrganizer.ini
// for the ones on this device.
func (in install) fixPaths() (bool, error) {
	data, err := os.ReadFile(in.ini())
	if err != nil {
		return false, err
	}
	text := string(data)

	moves := map[string]string{}
	if m := regexp.MustCompile(`(?m)^gamePath=@ByteArray\((.*)\)\r?$`).FindStringSubmatch(text); m != nil {
		moves[strings.ReplaceAll(m[1], `\\`, `\`)] = windowsPath(in.game)
	}
	if m := regexp.MustCompile(`(?m)^base_directory=(.*?)\r?$`).FindStringSubmatch(text); m != nil {
		moves[strings.ReplaceAll(m[1], "/", `\`)] = windowsPath(in.manager)
	}

	changed := text
	for from, to := range moves {
		if strings.EqualFold(from, to) || from == "" {
			continue
		}
		for _, form := range []func(string) string{
			func(s string) string { return strings.ReplaceAll(s, `\`, `\\`) },
			func(s string) string { return strings.ReplaceAll(s, `\`, "/") },
			func(s string) string { return s },
		} {
			changed = regexp.MustCompile(`(?i)`+regexp.QuoteMeta(form(from))).ReplaceAllLiteralString(changed, form(to))
		}
	}
	changed = editINI(changed, []setting{
		{"General", "gamePath", "@ByteArray(" + strings.ReplaceAll(windowsPath(in.game), `\`, `\\`) + ")"},
		{"Settings", "base_directory", strings.ReplaceAll(windowsPath(in.manager), `\`, "/")},
		{"Settings", "download_directory", strings.ReplaceAll(windowsPath(filepath.Join(in.manager, "downloads")), `\`, "/")},
		{"Plugins", `RootBuilder\exclusions`, rootBuilderExclusions},
	})
	if changed == text {
		return false, nil
	}
	return true, os.WriteFile(in.ini(), []byte(changed), 0o644)
}

// fixStewie has Root Builder place Stewie's Tweaks' DLL as a real file in the
// game folder. Under Wine the DLL can't find its INI through Mod Organizer's
// virtual file system otherwise.
func (in install) fixStewie() (bool, error) {
	mods := filepath.Join(in.manager, "mods")
	var dll string
	err := filepath.WalkDir(mods, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(mods, path)
		if d.IsDir() && strings.Count(rel, string(filepath.Separator)) >= 3 {
			return fs.SkipDir
		}
		if strings.EqualFold(d.Name(), "nvse_stewie_tweaks.dll") && strings.EqualFold(filepath.Base(filepath.Dir(path)), "plugins") &&
			strings.EqualFold(filepath.Base(filepath.Dir(filepath.Dir(path))), "nvse") && strings.Count(rel, string(filepath.Separator)) == 3 {
			dll = path
		}
		return nil
	})
	if err != nil {
		return false, err
	}
	if dll == "" {
		return false, nil
	}
	mod := filepath.Dir(filepath.Dir(filepath.Dir(dll)))
	target := filepath.Join(mod, "Root", "Data", "NVSE", "Plugins", "nvse_stewie_tweaks.dll")
	if err := copyFile(dll, target); err != nil {
		return false, err
	}
	return true, os.Rename(dll, dll+".mohidden")
}

func (in install) selectedProfile() (string, error) {
	data, err := os.ReadFile(in.ini())
	if err != nil {
		return "", err
	}
	m := regexp.MustCompile(`(?m)^selected_profile=@ByteArray\((.*)\)\r?$`).FindStringSubmatch(string(data))
	if m == nil {
		return "", fmt.Errorf("no profile selected in %s", in.ini())
	}
	return m[1], nil
}

func (in install) selectProfile(name string) error {
	return applyINI(in.ini(), []setting{{"General", "selected_profile", "@ByteArray(" + name + ")"}})
}

// setMod turns a mod on or off in a profile. New entries go to the top of
// the list, which is the highest priority.
func setMod(modlist, mod string, on bool) error {
	data, err := os.ReadFile(modlist)
	if err != nil {
		return err
	}
	eol := "\n"
	if strings.Contains(string(data), "\r\n") {
		eol = "\r\n"
	}
	lines := strings.Split(strings.TrimRight(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n"), "\n")
	state := "-"
	if on {
		state = "+"
	}
	found := false
	insert := len(lines)
	for i, l := range lines {
		if l == "+"+mod || l == "-"+mod {
			lines[i], found = state+mod, true
		}
		if insert == len(lines) && !strings.HasPrefix(l, "#") {
			insert = i
		}
	}
	if !found {
		if !on {
			return nil
		}
		lines = append(lines[:insert], append([]string{state + mod}, lines[insert:]...)...)
	}
	return os.WriteFile(modlist, []byte(strings.Join(lines, eol)+eol), 0o644)
}

// winningFile returns the highest-priority enabled mod's copy of rel,
// ignoring the mods this tool writes.
func (in install) winningFile(profile, rel string) (string, error) {
	data, err := os.ReadFile(filepath.Join(in.manager, "profiles", profile, "modlist.txt"))
	if err != nil {
		return "", err
	}
	for _, l := range strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
		mod, ok := strings.CutPrefix(l, "+")
		if !ok || mod == touchFixMod || mod == aoOffMod {
			continue
		}
		path, ok := findFold(filepath.Join(in.manager, "mods", mod), rel)
		if ok {
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

func copyDir(from, to string) error {
	return filepath.WalkDir(from, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(from, path)
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(to, rel), 0o755)
		}
		return copyFile(path, filepath.Join(to, rel))
	})
}
