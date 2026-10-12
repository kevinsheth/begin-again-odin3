package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const usage = `begin-again-odin3 sets up Begin Again on an Odin 3 running Armada.

  begin-again-odin3 setup --mo2 SRC --game SRC
                               copy Begin Again over and set everything up
  begin-again-odin3 settings   change graphics settings
  begin-again-odin3 steam      add or fix the Steam shortcut

Run any command with -h to see its options.
`

func main() {
	if len(os.Args) < 2 {
		fmt.Print(usage)
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "setup":
		err = runSetup(os.Args[2:])
	case "settings":
		err = runSettings(os.Args[2:])
	case "steam":
		err = runSteam(os.Args[2:])
	case "-h", "--help", "help":
		fmt.Print(usage)
		return
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", os.Args[1], usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

type options struct {
	dir, steamDir, dsoalZip, from, preset string
	mo2Src, gameSrc                       string
	verify                                bool
	g                                     graphics
	set                                   map[string]bool
}

func newFlags(name string, o *options, withGraphics bool) *flag.FlagSet {
	home, _ := os.UserHomeDir()
	fs := flag.NewFlagSet(name, flag.ExitOnError)
	fs.StringVar(&o.dir, "dir", filepath.Join(home, "Games", "BeginAgain"), "folder holding manager/ and game/")
	fs.StringVar(&o.steamDir, "steam-dir", filepath.Join(home, ".local", "share", "Steam"), "Steam folder")
	if !withGraphics {
		return fs
	}
	fs.StringVar(&o.preset, "preset", "", "start from balanced (900p), performance (720p) or quality (1080p)")
	fs.StringVar(&o.from, "from", "", "profile to copy the first time (default: the selected one)")
	fs.StringVar(&o.g.Resolution, "resolution", "", "like 1600x900, or default")
	fs.StringVar(&o.g.MSAA, "msaa", "", "anti-aliasing samples: 0, 2, 4, 8 or default")
	fs.StringVar(&o.g.Anisotropy, "anisotropy", "", "texture filtering: 1 to 16, or default")
	fs.StringVar(&o.g.AO, "ao", "", "ambient occlusion: on or off")
	fs.StringVar(&o.g.Shadows, "shadows", "", "on or off")
	fs.StringVar(&o.g.Water, "water", "", "water reflections and effects: on or off")
	fs.StringVar(&o.g.Grass, "grass", "", "on or off")
	fs.StringVar(&o.g.Blur, "blur", "", "depth of field and radial blur: on or off")
	fs.StringVar(&o.g.Detail, "detail", "", "decals and light fade distances: high or low")
	fs.StringVar(&o.g.DrawDistance, "draw-distance", "", "high or low")
	fs.StringVar(&o.g.TouchFix, "touch-fix", "", "stop touchscreen taps leaving controller mode: on or off")
	return fs
}

func (o options) install() install {
	return install{manager: filepath.Join(o.dir, "manager"), game: filepath.Join(o.dir, "game")}
}

// graphicsState merges the saved settings, the preset and any flags, in
// that order.
func (o options) graphicsState(in install) (state, error) {
	st, saved, err := in.loadState()
	if err != nil {
		return st, err
	}
	if !saved {
		st.Source = o.from
		if st.Source == "" {
			if st.Source, err = in.selectedProfile(); err != nil {
				return st, err
			}
		}
		if st.Source == odinProfile {
			return st, errors.New("the Odin profile exists without its settings file. Delete it in Mod Organizer, or pass --from with Begin Again's profile")
		}
		if o.preset == "" {
			o.preset = "balanced"
		}
	}
	if o.preset != "" {
		p, ok := presets[o.preset]
		if !ok {
			return st, fmt.Errorf("unknown preset %q. Pick balanced, performance or quality", o.preset)
		}
		st.Graphics = p
	}
	for _, f := range []struct{ from, to *string }{
		{&o.g.Resolution, &st.Graphics.Resolution}, {&o.g.MSAA, &st.Graphics.MSAA},
		{&o.g.Anisotropy, &st.Graphics.Anisotropy}, {&o.g.AO, &st.Graphics.AO},
		{&o.g.Shadows, &st.Graphics.Shadows}, {&o.g.Water, &st.Graphics.Water},
		{&o.g.Grass, &st.Graphics.Grass}, {&o.g.Blur, &st.Graphics.Blur},
		{&o.g.Detail, &st.Graphics.Detail}, {&o.g.DrawDistance, &st.Graphics.DrawDistance},
		{&o.g.TouchFix, &st.Graphics.TouchFix},
	} {
		if *f.from != "" {
			*f.to = strings.ToLower(*f.from)
		}
	}
	return st, nil
}

func runSetup(args []string) error {
	var o options
	fs := newFlags("setup", &o, true)
	fs.StringVar(&o.mo2Src, "mo2", "", "copy Begin Again's Mod Organizer folder from here (a path, or host:path over SSH)")
	fs.StringVar(&o.gameSrc, "game", "", "copy a clean Fallout New Vegas folder from here (a path, or host:path over SSH)")
	fs.BoolVar(&o.verify, "verify", false, "check every copied file's SHA-256 against the source")
	fs.StringVar(&o.dsoalZip, "dsoal-zip", "", "use a downloaded DSOAL_r695.zip instead of downloading it")
	fs.Parse(args)
	in := o.install()
	if gameRunning() {
		return errors.New("close the game and Mod Organizer first")
	}
	if err := fetchOnce(o.mo2Src, in.manager, []string{"downloads"}, o.verify); err != nil {
		return err
	}
	if err := fetchOnce(o.gameSrc, in.game, nil, o.verify); err != nil {
		return err
	}
	if err := in.check(); err != nil {
		return err
	}

	changed, err := in.fixPaths()
	if err != nil {
		return err
	}
	report(changed, "Pointed Mod Organizer at this device", "Mod Organizer paths already good")

	if changed, err = in.fixStewie(); err != nil {
		return err
	}
	report(changed, "Fixed Stewie's Tweaks for Linux", "Stewie's Tweaks already fixed")

	if changed, err = in.installDSOAL(o.dsoalZip); err != nil {
		return err
	}
	report(changed, "Installed DSOAL", "DSOAL already installed")

	st, err := o.graphicsState(in)
	if err != nil {
		return err
	}
	if err := in.applyGraphics(st); err != nil {
		return err
	}
	if err := setMod(filepath.Join(in.profileDir(odinProfile), "modlist.txt"), dsoalMod, true); err != nil {
		return err
	}
	fmt.Printf("Made the %q profile from %q with your settings\n", odinProfile, st.Source)

	return addSteam(o, in)
}

func runSettings(args []string) error {
	var o options
	fs := newFlags("settings", &o, true)
	fs.Usage = func() {
		fmt.Fprint(fs.Output(), "Change graphics settings. Anything you don't pass keeps its current value.\n\n")
		fs.PrintDefaults()
	}
	fs.Parse(args)
	in := o.install()
	if err := in.check(); err != nil {
		return err
	}
	st, err := o.graphicsState(in)
	if err != nil {
		return err
	}
	if err := in.applyGraphics(st); err != nil {
		return err
	}
	g := st.Graphics
	fmt.Printf("Settings: %s, MSAA %s, anisotropy %s, AO %s, shadows %s, water %s, grass %s, blur %s, detail %s, draw distance %s, touch fix %s\n",
		g.Resolution, g.MSAA, g.Anisotropy, g.AO, g.Shadows, g.Water, g.Grass, g.Blur, g.Detail, g.DrawDistance, g.TouchFix)
	return nil
}

func runSteam(args []string) error {
	var o options
	newFlags("steam", &o, false).Parse(args)
	in := o.install()
	if err := in.check(); err != nil {
		return err
	}
	return addSteam(o, in)
}

func addSteam(o options, in install) error {
	if _, err := os.Stat(filepath.Join(in.game, "dxvk.conf")); err != nil {
		fmt.Println("Heads up: game/dxvk.conf is missing, so there's no 60 FPS cap")
	}
	_, err := addShortcut(o.steamDir, shortcut{
		name:     "Fallout: Begin Again",
		exe:      filepath.Join(in.manager, "ModOrganizer.exe"),
		startDir: in.manager,
		options: `WINEDLLOVERRIDES="dsound=n,b" DXVK_CONFIG_FILE=` + filepath.Join(in.game, "dxvk.conf") +
			" /usr/libexec/armada/armada-game-launch %command%",
	})
	if errors.Is(err, errSteamRunning) {
		fmt.Println("\nAlmost done. Steam needs to be closed to add the shortcut.")
		fmt.Println("In Desktop Mode, right-click Steam's tray icon, pick Exit, then run:")
		fmt.Println("  begin-again-odin3 steam")
		return nil
	}
	if err != nil {
		return err
	}
	fmt.Println("Added Fallout: Begin Again to Steam. Start Steam and you're good to go")
	return nil
}

func report(changed bool, did, already string) {
	if changed {
		fmt.Println(did)
	} else {
		fmt.Println(already)
	}
}
