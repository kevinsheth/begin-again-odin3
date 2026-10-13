package main

import (
	"errors"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const (
	compatTool = "proton-experimental-arm64"

	// steamID64Base converts a 64-bit Steam ID to the account ID used for
	// userdata folder names.
	steamID64Base = 76561197960265728
)

var (
	errSteamRunning = errors.New("steam is running")

	loginUserPattern  = regexp.MustCompile(`"(\d{17})"\s*\{([^}]*)\}`)
	mostRecentPattern = regexp.MustCompile(`"MostRecent"\s*"1"`)
)

// steamRunning reports whether a Steam client process from root is running.
func steamRunning(root string) bool {
	procs, _ := filepath.Glob("/proc/[0-9]*/exe")
	for _, p := range procs {
		exe, err := os.Readlink(p)
		if err != nil {
			continue
		}
		exe = strings.TrimSuffix(exe, " (deleted)")
		name := filepath.Base(exe)
		if (name == "steam" || name == "steamwebhelper") && strings.HasPrefix(exe, root+"/") {
			return true
		}
	}
	return false
}

// steamUser returns the userdata folder name of the most recent login.
func steamUser(root string) (string, error) {
	dirs, _ := filepath.Glob(filepath.Join(root, "userdata", "[0-9]*"))
	var users []string
	for _, d := range dirs {
		if base := filepath.Base(d); base != "0" {
			users = append(users, base)
		}
	}
	if len(users) == 1 {
		return users[0], nil
	}
	if data, err := os.ReadFile(filepath.Join(root, "config", "loginusers.vdf")); err == nil {
		for _, m := range loginUserPattern.FindAllStringSubmatch(string(data), -1) {
			if mostRecentPattern.MatchString(m[2]) {
				id, err := strconv.ParseUint(m[1], 10, 64)
				if err != nil {
					return "", err
				}
				return strconv.FormatUint(id-steamID64Base, 10), nil
			}
		}
	}
	return "", fmt.Errorf("found %d Steam accounts and none is marked most recent", len(users))
}

type shortcut struct {
	name, exe, startDir, options string
}

// addShortcut creates or updates the non-Steam shortcut for s.exe and sets it
// to use ARM64 Proton. Steam must be closed or it overwrites both files.
func addShortcut(root string, s shortcut) error {
	if steamRunning(root) {
		return errSteamRunning
	}
	user, err := steamUser(root)
	if err != nil {
		return err
	}

	path := filepath.Join(root, "userdata", user, "config", "shortcuts.vdf")
	doc := &vdfNode{kind: vdfMap, nodes: []*vdfNode{{kind: vdfMap, key: "shortcuts"}}}
	data, err := os.ReadFile(path)
	switch {
	case err == nil:
		if doc, err = parseVDF(data); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		if err := backup(path); err != nil {
			return err
		}
	case !errors.Is(err, os.ErrNotExist):
		return err
	}
	list := doc.child("shortcuts")
	if list == nil {
		return fmt.Errorf("%s has no shortcuts list", path)
	}

	quoted := `"` + s.exe + `"`
	entry := findShortcut(list, s.exe)
	if entry == nil {
		entry = &vdfNode{kind: vdfMap, key: strconv.Itoa(len(list.nodes))}
		entry.setInt("appid", crc32.ChecksumIEEE([]byte(quoted+s.name))|0x80000000)
		list.nodes = append(list.nodes, entry)
	}
	entry.setString("AppName", s.name)
	entry.setString("Exe", quoted)
	entry.setString("StartDir", `"`+s.startDir+`"`)
	entry.setString("LaunchOptions", s.options)
	if entry.child("tags") == nil {
		entry.setString("icon", "")
		entry.setString("ShortcutPath", "")
		entry.setInt("IsHidden", 0)
		entry.setInt("AllowDesktopConfig", 1)
		entry.setInt("AllowOverlay", 1)
		entry.setInt("OpenVR", 0)
		entry.setInt("LastPlayTime", 0)
		entry.nodes = append(entry.nodes, &vdfNode{kind: vdfMap, key: "tags"})
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := writeFile(path, doc.encode()); err != nil {
		return err
	}
	id := strconv.FormatUint(entry.child("appid").num, 10)
	return setCompatTool(filepath.Join(root, "config", "config.vdf"), id)
}

func findShortcut(list *vdfNode, exe string) *vdfNode {
	for _, n := range list.nodes {
		if e := n.child("Exe"); e != nil && strings.Trim(e.str, `"`) == exe {
			return n
		}
	}
	return nil
}

func setCompatTool(path, id string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	text, err := mapCompatTool(string(data), id)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if err := backup(path); err != nil {
		return err
	}
	return writeFile(path, []byte(text))
}

// mapCompatTool sets app id to ARM64 Proton in the text of config.vdf.
func mapCompatTool(text, id string) (string, error) {
	lines := strings.Split(text, "\n")
	start := -1
	for i, l := range lines {
		if strings.TrimSpace(l) == `"CompatToolMapping"` {
			start = i
			break
		}
	}
	if start < 0 || start+1 >= len(lines) || strings.TrimSpace(lines[start+1]) != "{" {
		return "", errors.New("no CompatToolMapping section; set the compatibility tool in Steam instead")
	}
	indent := lines[start+1][:strings.Index(lines[start+1], "{")] + "\t"

	depth := 0
	for i := start + 1; i < len(lines); i++ {
		switch strings.TrimSpace(lines[i]) {
		case "{":
			depth++
		case "}":
			depth--
		}
		if depth == 0 {
			break
		}
		if depth != 1 || strings.TrimSpace(lines[i]) != `"`+id+`"` {
			continue
		}
		for j := i + 1; j < len(lines) && strings.TrimSpace(lines[j]) != "}"; j++ {
			if f := strings.Fields(lines[j]); len(f) == 2 && f[0] == `"name"` {
				lines[j] = strings.Replace(lines[j], f[1], `"`+compatTool+`"`, 1)
				return strings.Join(lines, "\n"), nil
			}
		}
	}

	block := []string{
		indent + `"` + id + `"`,
		indent + "{",
		indent + "\t\"name\"\t\t\"" + compatTool + "\"",
		indent + "\t\"config\"\t\t\"\"",
		indent + "\t\"priority\"\t\t\"250\"",
		indent + "}",
	}
	lines = append(lines[:start+2], append(block, lines[start+2:]...)...)
	return strings.Join(lines, "\n"), nil
}

// backup keeps a copy of path from before the first run.
func backup(path string) error {
	saved := path + backupSuffix
	if _, err := os.Stat(saved); err == nil {
		return nil
	}
	return copyFile(path, saved)
}
