# Begin Again on Odin 3

Play [Begin Again](https://www.nexusmods.com/newvegas/mods/79547) (the Tale of
Two Wastelands modlist) on an AYN Odin 3 running
[Armada OS](https://armadaos.dev/). You bring a finished Begin Again install,
and one tool copies it over and sets up everything else.

## What you need

- An Odin 3 on Armada with Steam signed in
- A Begin Again install that Wabbajack already built. That's two folders:
  the Mod Organizer folder (has `ModOrganizer.exe`) and the clean Fallout New
  Vegas folder it was built against
- Those folders somewhere the Odin can reach: a NAS over SSH, a USB drive, an
  SD card or the Odin itself
- About 75 GB free on the Odin

### Don't have Begin Again built yet?

Build it on any x86 Linux machine or Steam Deck with
[Jackify](https://github.com/Omni-guides/Jackify), which runs Wabbajack
modlists on Linux. It can't run on the Odin itself since the Odin is ARM. Do
the TTW install step from the
[Begin Again guide](https://www.nexusmods.com/newvegas/mods/79547?tab=description)
by hand, since Jackify doesn't automate that part for New Vegas lists yet.
A Windows PC with Wabbajack works too.

When it's done, leave the folders where they are or copy them to your NAS,
and point setup at them.

## Set it up

Switch to Desktop Mode, open Konsole and grab the tool:

```sh
curl -Lo ~/begin-again-odin3 https://github.com/kevinsheth/begin-again-odin3/releases/latest/download/begin-again-odin3
chmod +x ~/begin-again-odin3
```

Then point it at your two folders. This one pulls them from a NAS:

```sh
~/begin-again-odin3 setup --mo2 nas:/volume1/Games/BeginAgain --game "nas:/volume1/Games/Fallout New Vegas"
```

Paths on a USB drive or the Odin work too, like
`--mo2 "/run/media/armada/USB/Begin Again"`. For a NAS, `ssh nas` has to work
without a password first.

That's it. Setup will:

- Copy everything into `~/Games/BeginAgain`, skipping Wabbajack's downloads.
  If the copy gets cut off just run it again and it picks up where it stopped.
  Add `--verify` to check every file against the source
- Point Mod Organizer at the Odin's folders
- Fix Stewie's Tweaks so it can find its INI under Wine
- Install [DSOAL](https://github.com/kcat/dsoal). Without it Wine's audio
  eats almost a whole CPU core and the game stutters
- Make an `Odin` profile with settings that run well and a fix so touchscreen
  taps don't break the controller. Your original profile stays as is
- Add **Fallout: Begin Again** to Steam with ARM64 Proton

Steam has to be closed for that last part. If it's open, setup tells you to
exit Steam and run `~/begin-again-odin3 steam`.

## Play

Head back to Gaming Mode and hit **Play** on **Fallout: Begin Again**. When
Mod Organizer pops up, make sure it says **Begin Again** at the top right and
hit **Run**. The first launch takes a couple minutes before the menu shows up.

To quit, save and quit from the pause menu and let Mod Organizer close on its
own.

Begin Again uses Fallout 4 style controls:

| Button | Does |
| --- | --- |
| Select | First/third person |
| L1 | VATS |
| R1 | Bash |
| L3 | Sprint |
| R3 | Sneak or grab |
| Hold reload | Holster |

Every save is a `.fos` plus a `.nvse`, so keep the pairs together if you move
them around.

## Settings

Close the game, then change whatever you want. Anything you leave out stays
how it was.

```sh
~/begin-again-odin3 settings --preset performance
~/begin-again-odin3 settings --anisotropy 8 --resolution 1920x1080
```

| Preset | Resolution | Effects | How it ran on an Odin 3 Pro |
| --- | --- | --- | --- |
| `balanced` | 900p | Off | About 59 FPS. The default |
| `performance` | 720p | Off | Same FPS, a bit softer |
| `quality` | 1080p | Begin Again's | Hot and loud, FPS not measured |

| Flag | Values |
| --- | --- |
| `--preset` | `balanced`, `performance`, `quality` |
| `--resolution` | like `1600x900`, or `default` |
| `--msaa` | `0`, `2`, `4`, `8` or `default` |
| `--anisotropy` | `1` to `16`, or `default` |
| `--ao` | ambient occlusion `on` or `off` |
| `--shadows` | `on` or `off` |
| `--water` | reflections and effects `on` or `off` |
| `--grass` | `on` or `off` |
| `--blur` | depth of field and radial blur `on` or `off` |
| `--detail` | decals and light fade distance `high` or `low` |
| `--draw-distance` | `high` or `low` |
| `--touch-fix` | `on` or `off` |

`default`, `on` and `high` all mean Begin Again's own value. Begin Again
already caps the game at 60 FPS.

Change one thing at a time and play a bit. Good first upgrades from
`balanced` are `--anisotropy 8` and then 1080p. Leave draw distance for last
since the game's main thread was the bottleneck in testing, not the GPU.

Skip the launcher's Options screen. It rewrites the INIs and can undo your
settings.

## If something's off

| Problem | Try |
| --- | --- |
| Stuck on a loading screen | **Stop Game** in Steam and load an earlier save |
| Stuttering | `settings --preset performance` |
| Crackly audio | Run `setup` again to make sure DSOAL is in |
| Game quits at startup or controller stops working | Run `settings` once |

Expect the odd hitch on loading screens and in big fights.

## Building

```sh
go test .
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o begin-again-odin3 .
```

Pushing a `v*` tag builds a release.
