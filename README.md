# Begin Again on Odin 3

How to play [Begin Again](https://www.nexusmods.com/newvegas/mods/79547) on an
AYN Odin 3 running [Armada OS](https://armadaos.dev/). It runs through Begin
Again's own Mod Organizer under Proton, so you keep the full modlist and can
still update it later.

## What you need

- An Odin 3 on Armada with Steam set up
- A Windows PC for the Wabbajack install
- Fallout 3 and New Vegas on Steam
- About 75 GB free on the Odin

## Setup

### 1. Install it on your PC

Follow the [Begin Again guide](https://www.nexusmods.com/newvegas/mods/79547?tab=description)
all the way through, TTW step included. Get to the main menu once so you know
it works.

### 2. Copy it over

Close Mod Organizer, then copy two folders into `~/Games/BeginAgain` on the
Odin:

- Your Begin Again folder goes to `manager`
- Steam's `Fallout New Vegas` folder goes to `game`

You can leave out `manager/downloads`. It's only for installing and updating
and it's huge. Double check the sizes match when it's done.

### 3. Point Mod Organizer at the Odin

With Mod Organizer closed, open `manager/ModOrganizer.ini` and change the game
path:

```ini
gamePath=@ByteArray(Z:\\var\\home\\armada\\Games\\BeginAgain\\game)
```

Then find the `Begin Again` entry under `[customExecutables]` and change its
paths too. Yours might not be number `1`:

```ini
1\binary=Z:/var/home/armada/Games/BeginAgain/game/FalloutNV.exe
1\workingDirectory=Z:/var/home/armada/Games/BeginAgain/game
```

### 4. Fix Stewie's Tweaks

Stewie's Tweaks can't find its INI under Wine and complains at startup. Easy
fix. In `manager/mods/lStewieAl's Tweaks and Engine Fixes/`:

1. Copy `NVSE/plugins/nvse_stewie_tweaks.dll` into a new `Root/Data/NVSE/Plugins/` folder
2. Rename the original to `nvse_stewie_tweaks.dll.mohidden`

Then in `ModOrganizer.ini` swap Root Builder's exclusions for this so it
deploys the DLL but leaves the base game alone:

```ini
RootBuilder\exclusions="Saves,Morrowind.ini,Data Files,Data/*.bsa,Data/*.esm,Data/*.nam,Data/*.NAM,Data/*.txt,Data/Music,Data/Shaders,Data/Sound,Data/Video"
```

### 5. Add it to Steam

In Desktop Mode, **Add a Non-Steam Game** and pick `manager/ModOrganizer.exe`.
Then in its properties:

- Name it `Fallout: Begin Again`
- Start In: your `manager` folder
- Launch Options: `/usr/libexec/armada/armada-game-launch %command%`
- Compatibility: **Proton Experimental (ARM64)**

### 6. Pick your settings

Begin Again's settings are made for a gaming PC, so grab a preset. Open a
terminal on the Odin (Konsole in Desktop Mode works) and run:

```sh
git clone https://github.com/kevinsheth/begin-again-odin3 ~/begin-again-odin3
~/begin-again-odin3/scripts/odin-preset apply balanced
```

| Preset | Resolution | Effects | How it ran |
| --- | --- | --- | --- |
| `balanced` | 900p | Off | About 59 FPS. Start here |
| `performance` | 720p | Off | Same FPS, a bit softer |
| `quality` | 1080p | Begin Again defaults | Hot and loud, FPS not measured |

Each one becomes its own Mod Organizer profile. Your original stays untouched
and you can flip between them with the profile dropdown. All of them also set
the graphics card name the game needs to start and stop the touchscreen from
messing with your controls. Want to tweak things? See
[settings](docs/settings.md).

## Playing

Hit **Play** in Steam. When Mod Organizer pops up make sure it says
**Begin Again** at the top right and hit **Run**. The first launch takes a
couple minutes before the menu shows up.

To quit, save and quit from the pause menu, then let Mod Organizer close on
its own.

Begin Again uses Fallout 4 style controls:

| Button | Does |
| --- | --- |
| Select | First/third person |
| L1 | VATS |
| R1 | Bash |
| L3 | Sprint |
| R3 | Sneak or grab |
| Hold reload | Holster |

Your saves are in
`~/.local/share/Steam/steamapps/compatdata/<shortcut id>/pfx/drive_c/users/steamuser/Documents/My Games/FalloutNV/Saves/`.
Each one is a `.fos` and a `.nvse`, so keep the pairs together.

## If something's off

| Problem | Try |
| --- | --- |
| Stuck on a loading screen | **Stop Game** in Steam and load an earlier save |
| Stuttering | The `performance` preset |
| Game quits at startup | Apply a preset |
| Controller stops responding | Apply a preset |
| Stewie INI warning | Redo step 4 |

Expect the odd hitch on loading screens and in big fights.

## Optional: better audio

It works without it, but [DSOAL](https://github.com/kcat/dsoal) adds proper
3D positional sound. Make a mod called `DSOAL` with `Root/dsound.dll`
and `Root/dsoal-aldrv.dll` in it and turn it on.
