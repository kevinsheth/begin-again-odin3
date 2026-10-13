# Begin Again on Odin 3

A tool for running [Begin Again](https://www.nexusmods.com/newvegas/mods/79547),
the Tale of Two Wastelands modlist, on an AYN Odin 3 with
[Armada OS](https://armadaos.dev/). It copies an existing Begin Again install
to the Odin and configures it to run under Proton through Begin Again's own
Mod Organizer.

## Requirements

- An Odin 3 running Armada, with Steam signed in
- A Begin Again install built with Wabbajack: the Mod Organizer folder (the
  one with `ModOrganizer.exe`) and the clean Fallout New Vegas folder it was
  built against
- Access to both folders from the Odin, either over SSH or as a local path
  such as a USB drive
- About 75 GB of free space on the Odin

To build Begin Again on Linux, use
[Jackify](https://github.com/Omni-guides/Jackify) on an x86 machine or Steam
Deck. Jackify doesn't automate the TTW install step for New Vegas lists, so
follow that part of the
[Begin Again guide](https://www.nexusmods.com/newvegas/mods/79547?tab=description)
by hand. Wabbajack on Windows also works.

## Setup

In Desktop Mode, open Konsole and download the tool:

```sh
curl -Lo ~/begin-again-odin3 https://github.com/kevinsheth/begin-again-odin3/releases/latest/download/begin-again-odin3
chmod +x ~/begin-again-odin3
~/begin-again-odin3
```

The first run creates `~/begin-again-odin3.ini`. Set `mo2` and `game` under
`[source]`:

```ini
[source]
mo2 = nas:/volume1/Games/BeginAgain
game = nas:/volume1/Games/Fallout New Vegas
```

Use a local path for folders on a USB drive or already on the Odin. For SSH,
`ssh nas` must work without a password prompt.

Exit Steam, then run the tool again:

```sh
~/begin-again-odin3
```

It does the following:

- Copies both folders to `~/Games/BeginAgain`, leaving out Wabbajack's
  downloads. An interrupted copy resumes on the next run. Set `verify = true`
  to compare checksums with the source.
- Updates Mod Organizer's paths for the Odin.
- Has Root Builder deploy Stewie's Tweaks' DLL, which can't find its INI under
  Wine otherwise.
- Installs [DSOAL](https://github.com/kcat/dsoal). Wine's own DirectSound
  mixer uses most of a CPU core on the Odin, which causes audio crackling and
  frame drops.
- Sets the GPU name the game expects on the Odin. With the build machine's
  GPU name the game exits at startup.
- Adds a mod that stops touchscreen taps from switching the game out of
  controller mode.
- Adds a Steam shortcut, **Fallout: Begin Again**, that uses Proton
  Experimental (ARM64).

Running the tool again is safe. It skips anything already done.

## Playing

In Gaming Mode, launch **Fallout: Begin Again**. When Mod Organizer opens,
check that **Begin Again** is selected at the top right and press **Run**. The
first launch takes a few minutes to reach the main menu.

Quit from the pause menu and let Mod Organizer close before launching anything
else.

Begin Again uses Fallout 4's controller layout:

| Button | Action |
| --- | --- |
| Select | First/third person |
| L1 | VATS |
| R1 | Bash |
| L3 | Sprint |
| R3 | Sneak or grab |
| Hold reload | Holster |

Saves are pairs of `.fos` and `.nvse` files. Keep each pair together when
moving saves.

## Settings

Begin Again's own graphics settings are used by default. To change them, add
sections to `~/begin-again-odin3.ini` using the game's INI names and run the
tool again. They're written to `falloutprefs.ini` and `falloutcustom.ini` in
the selected Mod Organizer profile. Removing a line restores Begin Again's
value.

These are the settings tested on an Odin 3 Pro, which held about 59 FPS at
900p. Ambient occlusion was also off in that test, which is set in Vanilla Plus
AO's own INI rather than here.

```ini
[Display]
iSize W=1600
iSize H=900
iMultiSample=0
iWaterMultisamples=0
iMaxAnisotropy=1
iMaxDecalsPerFrame=2
iShadowFilter=0
iShadowMapResolution=256
fShadowLODStartFade=100
fSpecularLODStartFade=200
fLightLODStartFade=200
fDecalLOD1=500
fDecalLOD2=1000

[Grass]
b30GrassVS=0
bDrawShaderGrass=0
fGrassStartFadeDistance=0

[Imagespace]
bDoDepthOfField=0
iRadialBlurLevel=0

[Water]
bUseWaterReflections=0
bUseWaterRefractions=0
bUseWaterReflectionBlur=0
bUseWaterDisplacements=0
bUseWaterDepth=0
iWaterReflectWidth=256
iWaterReflectHeight=256
iWaterBlurAmount=0

[TerrainManager]
fBlockLoadDistanceLow=25000
fBlockLoadDistance=125000
fTreeLoadDistance=0
fSplitDistanceMult=0.75

[SpeedTree]
bForceFullLOD=0

[LOD]
fLODFadeOutMultActors=2.5
fLODFadeOutMultObjects=2.5
fLODFadeOutMultItems=3
```

Begin Again caps the frame rate at 60 in `dxvk.conf`. In testing, the game's
main thread was busier than the GPU.

Avoid the game launcher's Options screen, since it rewrites the INIs.

## Troubleshooting

| Problem | Fix |
| --- | --- |
| The game freezes on a loading screen | Use **Stop Game** in Steam and load an earlier save |
| Audio crackles or the frame rate drops sharply | Check that the DSOAL mod is enabled in Mod Organizer |
| The game exits at startup or the controller stops responding | Run the tool again |

## Building

```sh
go test .
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o begin-again-odin3 .
```

Pushing a `v*` tag publishes a release.
