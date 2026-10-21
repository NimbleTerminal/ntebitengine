# Kage lab

```sh
go run ./examples/shaders
```

Three animated effects use Ebitengine's [Kage fragment shaders](https://ebitengine.org/en/documents/shader.html)
through the existing `ntebitengine.Producer`:

- **Plasma:** a flowing procedural color field with three cosine palettes.
- **Rings:** three moving ripple sources with physically circular rings.
- **CRT:** plasma rendered into an intermediate GPU image, then sampled with
  barrel distortion, RGB separation, scanlines, a rolling band, and a vignette.

| Key | Action |
| --- | --- |
| `t` | Next effect |
| `p` | Next palette |
| `[` / `]` | Decrease / increase effect strength (25–200%) |
| Space | Pause / resume animation |
| `m` | Toggle Kitty graphics / half-block glyphs |
| `+` / `-` | Increase / decrease Kitty raster resolution |
| `q`, Esc, Ctrl+C | Quit |

`field.kage` and `crt.kage` are embedded in the executable. The draw callback
compiles them once with `ebiten.NewShader`; compilation errors flow through the
producer's `ErrorMsg` and shut down the example cleanly. No external assets or
shader downloads are needed.

`DrawRectShader` covers the exact `geo.Pixels` rectangle. Both shaders use
`//kage:unit pixels` and subtract image-region origins so coordinates work with
Ebitengine's texture atlas. The procedural shader uses `geo.DisplayAspect` for
physical proportions, including non-square pixels in half-block mode. Time,
palette, and strength arrive as uniforms; animation follows elapsed time rather
than counting frames.

The CRT source image is reused and recreated only when its pixel size changes.
It matches the shader rectangle, remains on the GPU, and is separate from the
destination. Only the final destination is read back by the producer. Every
fragment returns opaque alpha, including the curved border, so it is valid
premultiplied RGBA for the immutable framebuffer handoff. Effects with
transparency must return RGB multiplied by alpha.

Bubble Tea publishes immutable control snapshots through `atomic.Value`; the
drawing callback owns shaders, GPU images, and animation state. Pause freezes
time while still presenting frames, so other controls remain live. The FPS meter
counts committed picture frames, not GPU loop ticks or terminal acknowledgements.

Half-block mode shows broad color and motion well but has limited detail for
scanlines and thin rings. Higher Kitty resolution improves detail at the cost of
readback and terminal transfer work. Shader rendering does not remove those costs.

The browser demo runs the same program at `web/?demo=shaders` after
`sh scripts/build-site.sh`; see the repository README for serving instructions.
`go test ./examples/shaders` compiles both Kage programs to catch shader syntax
errors that an ordinary Go build cannot detect in embedded files.
