# ntui Kage lab

Floating ntui panels rendered by `nimbleterminal.dev/ntui/ebiten` over the shared
Kage lab. There is one Ebitengine loop, one composed destination, one final
readback, and the normal presentation-paced producer.

Controls: select effect/palette, drag strength, toggle pause or the light theme,
and type into Note. Drag title bars and scroll panels to reveal clipped widgets.
The telemetry panel plots **presented** FPS. Outside panels, clicking the scene
cycles palettes. Keyboard shortcuts are t/p, [/], Space, m, +/-, q; Escape clears
focus and Ctrl+C always quits. Text capture suppresses shortcuts; focused
widgets consume Space before scene pause. The two-row header is excluded from
UI hit coordinates.

At less than 6x12 raster pixels per cell, including glyph mode's 1x2 target,
the GPU omits UI text and a readable GlyphBackend control pane is reserved above
the scene. It uses the same widget declarations and logical cell input. The
picture renders only the scene beneath it. This also avoids putting ANSI UI text
over a Kitty image. Resize, mode, and resolution changes recompute that layout.
Viewport size changes restore initial panel positions so dragged titles remain
reachable.

Bubble Tea owns the context, widgets, animation phase, telemetry, and bound
values. After input and EndFrame it atomically publishes one struct containing
scene state, geometry, and `DrawList.Snapshot()` commands. DrawFunc loads once,
draws the shader, then renders the overlay into the same destination. A geometry
mismatch suppresses an obsolete overlay; the producer still owns invalidation.
GPU resources are callback-owned and reused for the process lifetime. The ntui
backend owns no GPU images; its font cache is managed by text/v2. Ebitengine
teardown handles engine caches and the scene resources at process exit.

## Running

The `ntui/ebiten` backend is included in ntui v0.2.0, required by this module.
Run from the repository root; no sibling checkout or temporary workspace is needed:

```sh
go -C examples run ./ntui
go -C examples test -race ./...
go -C examples vet ./...
sh scripts/build-site.sh
```

The browser build uses the checked-in `wasm.work` for the Bubble Tea browser fork.
The equivalent task commands are `task run-ntui` and `task build-wasm-site`.

Bundled Go Mono fonts use the [Go font license](../../web/licenses/go-fonts.txt).
