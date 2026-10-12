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

const compatTool = "proton-experimental-arm64"

var errSteamRunning = errors.New("Steam is running")

// steamRunning reports whether any Steam client process from root is alive.
func steamRunning(root string) bool {
	procs, _ := filepath.Glob("/proc/[0-9]*/exe")
	for _, p := range procs {
		exe, err := os.Readlink(p)
		if err != nil {
			continue
		}
		exe = strings.TrimSuffix(exe, " (deleted)")
		switch filepath.Base(exe) {
		case "steam", "steamwebhelper":
			if strings.HasPrefix(exe, root+string(filepath.Separator)) {
				return true
			}
		}
	}
	return false
}

// steamUser finds the account folder under userdata for the most recent login.
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
	data, err := os.ReadFile(filepath.Join(root, "config", "loginusers.vdf"))
	if err == nil {
		blocks := regexp.MustCompile(`"(\d{17})"\s*\{([^}]*)\}`).FindAllStringSubmatch(string(data), -1)
		for _, b := range blocks {
			if regexp.MustCompile(`"MostRecent"\s*"1"`).MatchString(b[2]) {
				id, _ := strconv.ParseUint(b[1], 10, 64)
				return strconv.FormatUint(id-76561197960265728, 10), nil
			}
		}
	}
	return "", fmt.Errorf("found %d Steam accounts and couldn't tell which one is yours", len(users))
}

type shortcut struct {
	name, exe, startDir, options string
}

// addShortcut creates or updates the non-Steam shortcut for exe and maps it
// to Armada's ARM64 Proton. Steam must be closed or it overwrites the files.
func addShortcut(root string, s shortcut) (uint32, error) {
	if steamRunning(root) {
		return 0, errSteamRunning
	}
	user, err := steamUser(root)
	if err != nil {
		return 0, err
	}

	path := filepath.Join(root, "userdata", user, "config", "shortcuts.vdf")
	doc := &vdfNode{kind: vdfMap, nodes: []*vdfNode{{kind: vdfMap, key: "shortcuts"}}}
	if data, err := os.ReadFile(path); err == nil {
		if doc, err = parseVDF(data); err != nil {
			return 0, fmt.Errorf("reading %s: %w", path, err)
		}
		if err := backup(path); err != nil {
			return 0, err
		}
	} else if !os.IsNotExist(err) {
		return 0, err
	}
	list := doc.child("shortcuts")
	if list == nil {
		return 0, fmt.Errorf("%s has no shortcuts list", path)
	}

	quoted := `"` + s.exe + `"`
	var entry *vdfNode
	for _, n := range list.nodes {
		if exe := n.child("Exe"); exe != nil && (exe.str == quoted || exe.str == s.exe) {
			entry = n
			break
		}
	}
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
		for _, k := range []string{"icon", "ShortcutPath"} {
			entry.setString(k, "")
		}
		entry.setInt("IsHidden", 0)
		entry.setInt("AllowDesktopConfig", 1)
		entry.setInt("AllowOverlay", 1)
		entry.setInt("OpenVR", 0)
		entry.setInt("LastPlayTime", 0)
		entry.nodes = append(entry.nodes, &vdfNode{kind: vdfMap, key: "tags"})
	}
	id := uint32(entry.child("appid").num)

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return 0, err
	}
	if err := os.WriteFile(path, doc.encode(), 0o644); err != nil {
		return 0, err
	}
	return id, setCompatTool(filepath.Join(root, "config", "config.vdf"), id)
}

func setCompatTool(path string, id uint32) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	text, err := mapCompatTool(string(data), strconv.FormatUint(uint64(id), 10))
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if err := backup(path); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(text), 0o644)
}

// mapCompatTool edits Steam's text config.vdf so the app uses ARM64 Proton.
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
		return "", errors.New("no CompatToolMapping section; set the compatibility tool in Steam by hand")
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
		if depth == 1 && strings.TrimSpace(lines[i]) == `"`+id+`"` {
			for j := i + 1; j < len(lines) && strings.TrimSpace(lines[j]) != "}"; j++ {
				if f := strings.Fields(lines[j]); len(f) == 2 && f[0] == `"name"` {
					lines[j] = strings.Replace(lines[j], f[1], `"`+compatTool+`"`, 1)
					return strings.Join(lines, "\n"), nil
				}
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

func backup(path string) error {
	saved := path + ".before-begin-again"
	if _, err := os.Stat(saved); err == nil {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return os.WriteFile(saved, data, 0o644)
}
