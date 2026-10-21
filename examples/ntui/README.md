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

## Developing before the ntui backend is published

The checked-in minimum ntui version predates the backend. Until a version with
`ntui/ebiten` is available, use checkouts of these two changes in a **temporary**
Go workspace. Do not add local replacements to go.mod or commit a workspace:

Run from ntebitengine with the updated ntui checkout next to it:

```sh
nte_dir="$PWD"
ntui_dir="$(cd ../ntui && pwd)"
workspace_dir=$(mktemp -d)
(cd "$workspace_dir" && go work init "$nte_dir" "$ntui_dir")
GOWORK="$workspace_dir/go.work" go run ./examples/ntui
GOWORK="$workspace_dir/go.work" go test -race ./...
GOWORK="$workspace_dir/go.work" go vet ./...

# Add the same browser-only Bubble Tea replacement as wasm.work:
cp "$workspace_dir/go.work" "$workspace_dir/wasm.work"
GOWORK="$workspace_dir/wasm.work" go work edit \
  -replace=charm.land/bubbletea/v2=github.com/neomantra/bubbletea/v2@v2.0.0-20261009201241-c397c32a5074
NTEBITENGINE_WASM_WORK="$workspace_dir/wasm.work" sh scripts/build-site.sh
```

After publication, update the ntui requirement to that real version; then normal
`task run-ntui` and `task build-wasm-site` need no temporary workspace.
No release or invented future version is required to review these changes.

Bundled Go Mono fonts use the [Go font license](../../web/licenses/go-fonts.txt).
