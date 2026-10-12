package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	odinProfile = "Odin"
	stateFile   = "begin-again-odin3.json"
	touchFixMod = "Odin Touchscreen Fix"
	aoOffMod    = "Odin Ambient Occlusion Off"
	dsoalMod    = "DSOAL"
	// The name DXVK reports on the Odin. With the PC's GPU name the game exits during startup.
	odinDevice = `"AMD Radeon RX 6700 XT"`
)

// graphics holds every setting the tool manages. "default" and "on" leave
// Begin Again's own value alone.
type graphics struct {
	Resolution   string `json:"resolution"`
	MSAA         string `json:"msaa"`
	Anisotropy   string `json:"anisotropy"`
	AO           string `json:"ao"`
	Shadows      string `json:"shadows"`
	Water        string `json:"water"`
	Grass        string `json:"grass"`
	Blur         string `json:"blur"`
	Detail       string `json:"detail"`
	DrawDistance string `json:"draw_distance"`
	TouchFix     string `json:"touch_fix"`
}

var presets = map[string]graphics{
	"balanced":    {"1600x900", "0", "1", "off", "off", "off", "off", "off", "low", "low", "on"},
	"performance": {"1280x720", "0", "1", "off", "off", "off", "off", "off", "low", "low", "on"},
	"quality":     {"1920x1080", "default", "default", "on", "on", "on", "on", "on", "high", "high", "on"},
}

func (g graphics) validate() error {
	if g.Resolution != "default" {
		w, h, ok := strings.Cut(g.Resolution, "x")
		if _, err := strconv.Atoi(w); err != nil || !ok {
			return fmt.Errorf("resolution should look like 1600x900, not %q", g.Resolution)
		}
		if _, err := strconv.Atoi(h); err != nil {
			return fmt.Errorf("resolution should look like 1600x900, not %q", g.Resolution)
		}
	}
	for name, v := range map[string]string{"msaa": g.MSAA, "anisotropy": g.Anisotropy} {
		if _, err := strconv.Atoi(v); err != nil && v != "default" {
			return fmt.Errorf("%s should be a number or default, not %q", name, v)
		}
	}
	for name, v := range map[string]string{"ao": g.AO, "shadows": g.Shadows, "water": g.Water, "grass": g.Grass, "blur": g.Blur, "touch-fix": g.TouchFix} {
		if v != "on" && v != "off" {
			return fmt.Errorf("%s should be on or off, not %q", name, v)
		}
	}
	for name, v := range map[string]string{"detail": g.Detail, "draw-distance": g.DrawDistance} {
		if v != "high" && v != "low" {
			return fmt.Errorf("%s should be high or low, not %q", name, v)
		}
	}
	return nil
}

// iniSettings turns g into the game INI values that differ from Begin Again's.
func (g graphics) iniSettings() []setting {
	s := []setting{{"Display", "sD3DDevice", odinDevice}}
	add := func(section string, kv ...string) {
		for i := 0; i < len(kv); i += 2 {
			s = append(s, setting{section, kv[i], kv[i+1]})
		}
	}
	if g.Resolution != "default" {
		w, h, _ := strings.Cut(g.Resolution, "x")
		add("Display", "iSize W", w, "iSize H", h)
	}
	if g.MSAA != "default" {
		add("Display", "iMultiSample", g.MSAA, "iWaterMultisamples", g.MSAA)
	}
	if g.Anisotropy != "default" {
		add("Display", "iMaxAnisotropy", g.Anisotropy)
	}
	if g.Shadows == "off" {
		add("Display", "iShadowFilter", "0", "iShadowMapResolution", "256", "fShadowLODStartFade", "100")
	}
	if g.Detail == "low" {
		add("Display", "iMaxDecalsPerFrame", "2", "fSpecularLODStartFade", "200", "fLightLODStartFade", "200",
			"fDecalLOD1", "500", "fDecalLOD2", "1000")
	}
	if g.Grass == "off" {
		add("Grass", "b30GrassVS", "0", "bDrawShaderGrass", "0", "fGrassStartFadeDistance", "0")
	}
	if g.Blur == "off" {
		add("Imagespace", "bDoDepthOfField", "0", "iRadialBlurLevel", "0")
	}
	if g.Water == "off" {
		add("Water", "bUseWaterReflections", "0", "bUseWaterRefractions", "0", "bUseWaterReflectionBlur", "0",
			"bUseWaterDisplacements", "0", "bUseWaterDepth", "0", "iWaterReflectWidth", "256",
			"iWaterReflectHeight", "256", "iWaterBlurAmount", "0")
	}
	if g.DrawDistance == "low" {
		add("TerrainManager", "fBlockLoadDistanceLow", "25000", "fBlockLoadDistance", "125000",
			"fTreeLoadDistance", "0", "fSplitDistanceMult", "0.75")
		add("SpeedTree", "bForceFullLOD", "0")
		add("LOD", "fLODFadeOutMultActors", "2.5", "fLODFadeOutMultObjects", "2.5", "fLODFadeOutMultItems", "3")
	}
	return s
}

type state struct {
	Source   string   `json:"source"`
	Graphics graphics `json:"graphics"`
}

func (in install) profileDir(name string) string { return filepath.Join(in.manager, "profiles", name) }

func (in install) loadState() (state, bool, error) {
	data, err := os.ReadFile(filepath.Join(in.profileDir(odinProfile), stateFile))
	if os.IsNotExist(err) {
		return state{}, false, nil
	}
	if err != nil {
		return state{}, false, err
	}
	var st state
	return st, true, json.Unmarshal(data, &st)
}

// applyGraphics builds the Odin profile from its source profile's INIs plus
// g, then selects it. The source profile is never changed.
func (in install) applyGraphics(st state) error {
	if err := st.Graphics.validate(); err != nil {
		return err
	}
	odin := in.profileDir(odinProfile)
	src := in.profileDir(st.Source)
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("profile %q is missing", st.Source)
	}
	if _, err := os.Stat(odin); os.IsNotExist(err) {
		if err := copyDir(src, odin); err != nil {
			return err
		}
	}
	for _, f := range []string{"falloutprefs.ini", "falloutcustom.ini"} {
		if _, err := os.Stat(filepath.Join(src, f)); err != nil {
			continue
		}
		if err := copyFile(filepath.Join(src, f), filepath.Join(odin, f)); err != nil {
			return err
		}
		if err := applyINI(filepath.Join(odin, f), st.Graphics.iniSettings()); err != nil {
			return err
		}
	}
	if err := applyINI(filepath.Join(odin, "settings.ini"), []setting{{"General", "LocalSettings", "true"}}); err != nil {
		return err
	}

	modlist := filepath.Join(odin, "modlist.txt")
	overrides := []struct {
		mod, ini string
		on       bool
		values   []setting
	}{
		{touchFixMod, "nvse_stewie_tweaks.ini", st.Graphics.TouchFix == "on",
			[]setting{{"Tweaks", "bToggleControllerIfAttackPressed", "0"}}},
		{aoOffMod, "VanillaPlusAO.ini", st.Graphics.AO == "off",
			[]setting{{"Exterior", "bEnabled", "0"}, {"Interior", "bEnabled", "0"}}},
	}
	for _, o := range overrides {
		if o.on {
			rel := filepath.Join("NVSE", "Plugins", o.ini)
			from, err := in.winningFile(odinProfile, rel)
			if err != nil {
				return err
			}
			to := filepath.Join(in.manager, "mods", o.mod, rel)
			if err := copyFile(from, to); err != nil {
				return err
			}
			if err := applyINI(to, o.values); err != nil {
				return err
			}
		}
		if err := setMod(modlist, o.mod, o.on); err != nil {
			return err
		}
	}

	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(odin, stateFile), append(data, '\n'), 0o644); err != nil {
		return err
	}
	return in.selectProfile(odinProfile)
}
