# Settings

Close the game and Mod Organizer before running any of this.

## Presets

```sh
scripts/odin-preset list
scripts/odin-preset apply balanced
```

That copies whatever profile you have selected, applies the preset and
switches you to the copy. A few options if you need them:

| Option | Does |
| --- | --- |
| `--from PROFILE` | Copy a different profile |
| `--name PROFILE` | Pick the new profile's name |
| `--keep-selection` | Don't switch to it |
| `--mo2 DIR` | Your Mod Organizer folder, if the script can't find it |

"Effects off" in the presets means no MSAA, ambient occlusion, shadows, water
reflections, grass or depth of field, plus shorter draw distances. Begin Again
already caps things at 60 FPS.

## Make your own

Copy one, edit it and apply it by name:

```sh
cp presets/balanced.ini presets/sharper.ini
scripts/odin-preset apply sharper --from "Odin Balanced"
```

Presets are just the game's own INI sections and keys. The script writes them
to both `falloutprefs.ini` and `falloutcustom.ini` since the custom one wins
when they disagree. Put `AmbientOcclusion=0` under `[Odin]` to turn off
ambient occlusion.

Change one thing at a time and play a bit before the next. Nice first upgrades
from `balanced` are `iMaxAnisotropy=8` for sharper textures and then 1080p.
Save draw distances for last because the game's main thread was the
bottleneck in testing, not the GPU.

## Skip these

- Turning off Dynamic Reflections with `bEnable`. Shiny stuff starts flashing
  white, cyan and purple
- The launcher's Options screen. It rewrites the INIs and can undo a preset
- Editing profiles with the game open. Your changes can get overwritten when
  it closes

## Tests

`tests/odin-preset.sh` checks the script. It needs Linux, so run it on the
Odin.
