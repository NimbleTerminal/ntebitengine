# Sprite parade

```sh
go -C examples run ./sprites
```

Three lanes of pixel robots walk across a starry background. Each robot uses
one of four animation frames from the same transparent sprite sheet, tinted
with `ColorScale`. The middle lane mirrors sprites with a negative X scale.

The header shows presented FPS, sampled every half second from ntpicture's
`PresentedMsg` events using actual elapsed time. It measures committed frames,
not Ebitengine loop ticks or terminal display acknowledgements. The meter falls
to zero when no frames are committed; pausing animation still renders frames.

| Key | Action |
| --- | --- |
| Space | Pause/resume movement and animation |
| `[` / `]` | Remove/add three robots (3–60) |
| `m` | Toggle Kitty/glyph presentation when Kitty is available |
| `+` / `-` | Adjust Kitty raster resolution |
| `q`, Esc, Ctrl+C | Quit |

[scene.go](scene.go) generates a four-frame 16×16 sprite sheet, uploads it once
inside the drawing callback, and caches `SubImage` views into the sheet. Every
robot is drawn with `DrawImage`; the cached GPU images survive framebuffer
resizes. Replace `spriteSheet()` with a decoded PNG to use your own artwork.

Animation uses elapsed time and runs only in the Ebitengine callback. Bubble Tea
publishes an immutable control snapshot through `atomic.Value`; the renderer
never accesses the Tea model or Picture. Pausing freezes the scene clock while
the producer continues its normal presentation-paced rendering.

The scene uses `RasterGeometry.DisplayAspect` to preserve sprite proportions in
both Kitty and glyph modes. Nearest-neighbor filtering keeps the pixel art crisp;
glyph mode necessarily loses detail at its much smaller raster resolution.
