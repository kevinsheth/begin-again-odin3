package main

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	dsoalMod    = "DSOAL"
	touchFixMod = "Odin Touchscreen Fix"

	// odinDevice is the GPU name DXVK reports on the Odin. The game exits at
	// startup if sD3DDevice names a different GPU.
	odinDevice = `"AMD Radeon RX 6700 XT"`

	// backupSuffix marks the copy of a file as it was before the first run.
	backupSuffix = ".before-begin-again"
)

func (in install) profileDir(name string) string {
	return filepath.Join(in.manager, "profiles", name)
}

// applySettings writes the config's game settings to the profile's INIs,
// starting from the INIs as they were before the first run so removed
// settings go back to Begin Again's values. It also turns on DSOAL and the
// touchscreen fix.
func (in install) applySettings(profile string, c config) error {
	dir := in.profileDir(profile)
	values := append([]setting{{"Display", "sD3DDevice", odinDevice}}, c.gameINI...)
	for _, name := range []string{"falloutprefs.ini", "falloutcustom.ini"} {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err != nil {
			continue
		}
		if err := backup(path); err != nil {
			return err
		}
		if err := copyFile(path+backupSuffix, path); err != nil {
			return err
		}
		if err := applyINI(path, values); err != nil {
			return err
		}
	}
	if err := applyINI(filepath.Join(dir, "settings.ini"), []setting{{"General", "LocalSettings", "true"}}); err != nil {
		return err
	}

	modlist := filepath.Join(dir, "modlist.txt")
	if err := setMod(modlist, dsoalMod, true); err != nil {
		return err
	}
	if c.touchscreenFix {
		if err := in.writeTouchFix(profile); err != nil {
			return err
		}
	}
	return setMod(modlist, touchFixMod, c.touchscreenFix)
}

// writeTouchFix makes a mod with a copy of Stewie's Tweaks' INI that stops
// touchscreen taps from switching the game to keyboard and mouse mode.
func (in install) writeTouchFix(profile string) error {
	rel := filepath.Join("NVSE", "Plugins", "nvse_stewie_tweaks.ini")
	from, err := in.winningFile(profile, rel)
	if err != nil {
		return fmt.Errorf("touchscreen fix: %w", err)
	}
	to := filepath.Join(in.manager, "mods", touchFixMod, rel)
	if err := copyFile(from, to); err != nil {
		return err
	}
	return applyINI(to, []setting{{"Tweaks", "bToggleControllerIfAttackPressed", "0"}})
}
