// Command begin-again-odin3 copies a Begin Again install to an AYN Odin 3
// running Armada OS and configures it to run there.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	home, _ := os.UserHomeDir()
	configPath := flag.String("config", defaultConfigPath(), "settings file")
	steamDir := flag.String("steam-dir", filepath.Join(home, ".local", "share", "Steam"), "Steam folder")
	flag.Usage = func() {
		fmt.Fprintln(flag.CommandLine.Output(), "Usage: begin-again-odin3 [flags]")
		fmt.Fprintln(flag.CommandLine.Output(), "\nCopies Begin Again to this Odin and sets it up using the settings file.")
		flag.PrintDefaults()
	}
	flag.Parse()

	if err := run(*configPath, *steamDir); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(configPath, steamDir string) error {
	c, err := loadConfig(configPath)
	if errors.Is(err, errNewConfig) {
		fmt.Printf("Created %s. Fill in [source], then run begin-again-odin3 again.\n", configPath)
		return nil
	}
	if err != nil {
		return fmt.Errorf("%s: %w", configPath, err)
	}

	in := install{manager: filepath.Join(c.dir, "manager"), game: filepath.Join(c.dir, "game")}
	if gameRunning() {
		return errGameRunning
	}
	if err := fetchOnce(c.mo2, in.manager, []string{"downloads"}, c.verify); err != nil {
		return err
	}
	if err := fetchOnce(c.game, in.game, nil, c.verify); err != nil {
		return err
	}
	if err := in.check(); err != nil {
		return err
	}

	steps := []struct {
		name string
		run  func() (bool, error)
	}{
		{"Mod Organizer paths", in.fixPaths},
		{"Stewie's Tweaks DLL", in.fixStewie},
		{"DSOAL", in.installDSOAL},
	}
	for _, s := range steps {
		changed, err := s.run()
		if err != nil {
			return fmt.Errorf("%s: %w", s.name, err)
		}
		if changed {
			fmt.Println("Set up", s.name)
		}
	}

	profile, err := in.selectedProfile()
	if err != nil {
		return err
	}
	if err := in.applySettings(profile, c); err != nil {
		return err
	}
	fmt.Printf("Applied settings to the %s profile\n", profile)

	return addSteam(steamDir, in)
}

func addSteam(steamDir string, in install) error {
	conf := filepath.Join(in.game, "dxvk.conf")
	if _, err := os.Stat(conf); err != nil {
		fmt.Println("Warning: game/dxvk.conf is missing, so the 60 FPS cap won't apply")
	}
	err := addShortcut(steamDir, shortcut{
		name:     "Fallout: Begin Again",
		exe:      filepath.Join(in.manager, "ModOrganizer.exe"),
		startDir: in.manager,
		options:  `WINEDLLOVERRIDES="dsound=n,b" DXVK_CONFIG_FILE=` + conf + " /usr/libexec/armada/armada-game-launch %command%",
	})
	if errors.Is(err, errSteamRunning) {
		fmt.Println("Steam is running, so the shortcut wasn't added. Exit Steam from its tray icon in Desktop Mode and run begin-again-odin3 again.")
		return nil
	}
	if err != nil {
		return err
	}
	fmt.Println("Added Fallout: Begin Again to Steam")
	return nil
}
