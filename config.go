package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const configTemplate = `; Settings for begin-again-odin3. Run begin-again-odin3 again after changing
; anything here.

[source]
; Begin Again's Mod Organizer folder and the clean Fallout New Vegas folder it
; was built against. Use a local path or host:path to copy over SSH.
mo2 =
game =
; Compare the SHA-256 of every copied file with the source
verify = false

[odin]
; Where Begin Again is installed on the Odin
dir = ~/Games/BeginAgain
; Keep touchscreen taps from switching the game out of controller mode
touchscreen_fix = true

; Any other section is written to falloutprefs.ini and falloutcustom.ini in
; the selected Mod Organizer profile. Remove a line to go back to Begin Again's
; value. For example:
;
; [Display]
; iSize W=1600
; iSize H=900
; iMultiSample=0
`

// config is the parsed settings file.
type config struct {
	mo2, game      string
	verify         bool
	dir            string
	touchscreenFix bool
	gameINI        []setting
}

func defaultConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "begin-again-odin3.ini")
}

// loadConfig reads path. If it doesn't exist, it writes the template there
// and returns errNewConfig.
func loadConfig(path string) (config, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		if err := os.WriteFile(path, []byte(configTemplate), 0o644); err != nil {
			return config{}, err
		}
		return config{}, errNewConfig
	}
	if err != nil {
		return config{}, err
	}
	return parseConfig(string(data))
}

var errNewConfig = errors.New("new config file")

func parseConfig(text string) (config, error) {
	c := config{touchscreenFix: true}
	for _, s := range parseINI(text) {
		switch strings.ToLower(s.section) {
		case "source":
			switch strings.ToLower(s.key) {
			case "mo2":
				c.mo2 = s.value
			case "game":
				c.game = s.value
			case "verify":
				c.verify = s.value == "true"
			default:
				return c, fmt.Errorf("unknown setting %q in [source]", s.key)
			}
		case "odin":
			switch strings.ToLower(s.key) {
			case "dir":
				c.dir = expandHome(s.value)
			case "touchscreen_fix":
				c.touchscreenFix = s.value == "true"
			default:
				return c, fmt.Errorf("unknown setting %q in [odin]", s.key)
			}
		default:
			c.gameINI = append(c.gameINI, s)
		}
	}
	if c.dir == "" {
		return c, errors.New("dir in [odin] is empty")
	}
	return c, nil
}

func expandHome(path string) string {
	rest, ok := strings.CutPrefix(path, "~/")
	if !ok {
		return path
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, rest)
}
