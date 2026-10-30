# ntebitengine

Draw with Ebitengine and present a live framebuffer in a Bubble Tea terminal
application through [ntpicture](https://github.com/NimbleTerminal/ntpicture).
Sprites, Kage shaders, vector paths, and text render into an offscreen GPU image
at the picture's exact raster size. The adapter reads back each requested frame
and hands it to ntpicture for Kitty graphics or half-block glyph presentation.

Use this package when you want Ebitengine's drawing APIs. Use
[ntgpuimage](https://github.com/NimbleTerminal/ntgpuimage) when you want its pure
offscreen WebGPU renderer and `*image.NRGBA` output. Both can feed ntpicture;
[nt3d](https://github.com/NimbleTerminal/nt3d) can sit above either renderer.

## Install and run

Requires Go 1.26.8 or later and desktop graphics
support. Ebitengine is pinned to v2.10.5 for hidden-window support.

```sh
go get nimbleterminal.dev/ntebitengine
go run ./examples/bounce
```

The [bounce example](examples/bounce/main.go) animates a vector ball, adapts to
terminal resizing, and supports `m` to switch modes, `+`/`-` to change Kitty
resolution, and `q` to quit. It uses physical display aspect to keep the ball
round even when glyph raster pixels are not square.

```sh
go run ./examples/sprites
```

The [sprite parade](examples/sprites) animates tinted pixel robots from a shared
four-frame sprite sheet. It demonstrates `SubImage`, `DrawImage`, nearest-neighbor
scaling, horizontal flips, and independent animation phases. Press `space` to
pause, `[`/`]` to remove/add robots, `m` to switch modes, `+`/`-` for resolution,
and `q` to quit. The sheet is generated in Go; no assets or downloads are needed.

```sh
go run ./examples/text
```

The [text lab](examples/text) uses Ebitengine's `text/v2` API with bundled Go
Sans, Bold, and Mono fonts. Press `t` to cycle specimen, wrapped paragraph, and
animated text scenes; `f` for fonts, `a` for alignment, `n` for sample text, and
`[`/`]` for type size. It includes the presented-FPS meter and the same pause,
mode, and resolution controls as the sprite demo. Kitty mode at higher resolution
makes small type clearer; half-block mode shows the limits of a coarse raster.

```sh
go run ./examples/shaders
```

The [Kage lab](examples/shaders) renders plasma, neon ripple rings, and a CRT
effect with embedded fragment shaders. Press `t` for effects, `p` for palettes,
and `[`/`]` for effect strength. Space pauses animation; mode, resolution, quit,
and presented-FPS behavior match the other labs. The CRT scene demonstrates a
second shader pass sampling an intermediate GPU image, with one final readback.

```sh
go run ./examples/loops
```

The [Loop lab](examples/loops) accompanies the
[event-loop guide](docs/hugo/content/guide/event-loops.md). Bubble Tea advances a
simulation and publishes immutable snapshots; Ebitengine draws the latest one.
Press `l` to slow the drawing callback deliberately and watch Tea remain active.
Space pauses simulation, `[`/`]` changes speed, and `r` resets the scene. Separate
tick, callback, submission, and presentation counters explain the handoff.

## Guide

The [guide](docs/hugo/content/guide/event-loops.md) covers main-thread startup,
ownership, snapshots, simulation timing, presentation pacing, resize cancellation,
and shutdown. The [companion walkthrough](docs/hugo/content/guide/loop-lab.md)
explains what to try in the demo.

Like nt3d, this repository builds a Hugo Book guide at the demo site's `docs/`
path. Markdown lives under `docs/hugo/content`; its theme is pinned in a separate
Hugo module. Build just the guide with `sh scripts/build-docs.sh` (or
`task build-docs`) using Hugo Extended 0.167.0. Serve `web/` as described below
and open `http://localhost:8000/web/docs/`.

## Browser demo page

The `web/` page follows the ntsvg/ntpdf demo layout and runs all five examples locally
in the browser through booba. It includes demo navigation, clickable controls,
Kitty/glyph switching, a renderer HUD, and restart links. Ebitengine uses WebGL
on an offscreen canvas; the visible canvas is booba's terminal.

Build and serve it with Go, Node.js, Hugo Extended 0.167.0, and Python installed:

```sh
sh scripts/build-site.sh
python3 -m http.server 8000 --bind 127.0.0.1
# Open http://localhost:8000/web/
```

With [Task](https://taskfile.dev/) installed, `task serve-wasm-site` does both.
`task check-site` checks the generated assets and WASM binaries. Generated
runtime files and binaries are ignored by Git.

Browser builds select the pinned Bubble Tea browser fork through `wasm.work`;
native builds still use upstream Bubble Tea. The booba bridge is confined to the
example runner. Browser graphics require WebGL and may be throttled in background
tabs. Restart reloads the page because Ebitengine allows only one loop per process.

The GitHub Pages workflow builds and deploys `web/` on pushes to `main`, version
tags, published releases, or manual dispatch. Select **GitHub Actions** as the
repository's Pages source. Its expected project URL is
`https://nimbleterminal.github.io/ntebitengine/` once deployed.

## Bubble Tea integration

Create the picture with `StableID: true`, `FitFill`, and `AutoMode: true` for
terminal capability discovery. Leave `ClearOnReplace` disabled. The producer
sets `FitFill` in `Init`, but StableID is a construction-time Picture option
and must be supplied by the caller.

```go
pic := ntpicture.New(ntpicture.Options{
    AutoMode: true,
    StableID: true,
    Render: ntpicture.Config{Fit: ntpicture.FitFill},
})
producer := ntebitengine.New(pic, func(dst *ebiten.Image, geo ntpicture.RasterGeometry) error {
    dst.Fill(color.RGBA{R: 30, G: 80, B: 140, A: 255})
    // Draw sprites, vectors, text, or shaders here.
    return nil
}, ntebitengine.Options{ManualRun: true})
```

The host owns the picture's lifecycle and geometry. Forward **every message**
to `pic.Update`, then `producer.Update`; the producer does not forward messages
to the picture itself. Apply layout and mode/resolution changes first, and run
every returned command:

```go
func (m *model) Init() tea.Cmd {
    return tea.Batch(m.pic.Init(), m.producer.Init())
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    var cmds []tea.Cmd
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        cmds = append(cmds, m.pic.SetSize(msg.Width, msg.Height))
    case tea.KeyPressMsg:
        if msg.String() == "q" || msg.String() == "ctrl+c" {
            return m, tea.Sequence(m.producer.Close(), m.pic.Close(), tea.Quit)
        }
    case ntebitengine.ErrorMsg:
        if msg.Owner == m.producer {
            // Record msg.Err in your model, or close and quit.
        }
    }
    cmds = append(cmds, m.pic.Update(msg))
    cmds = append(cmds, m.producer.Update(msg))
    return m, tea.Batch(cmds...)
}

func (m *model) View() tea.View {
    v := tea.NewView(m.pic.View())
    v.AltScreen = true
    return v
}
```

### Main-thread startup

[Ebitengine documents](https://pkg.go.dev/github.com/hajimehoshi/ebiten/v2#RunGame)
that `RunGame` must run on the main thread, and must not be called more than once
per process. Use `ManualRun: true` and run Bubble Tea in a goroutine while
`Producer.Run` occupies `main`. This is the portable path and the one used by
the example:

```go
func main() {
    m := newModel() // constructs producer with ManualRun: true
    done := make(chan error, 1)
    go func() {
        _, err := tea.NewProgram(m).Run()
        m.producer.Close()() // also handles Tea failing before Init
        _ = m.pic.Close()   // local cleanup after Tea has exited
        done <- err
    }()
    loopErr := m.producer.Run()
    if err := errors.Join(loopErr, <-done); err != nil {
        fmt.Fprintln(os.Stderr, err)
    }
}
```

Handle `ntebitengine.ErrorMsg` by closing/quitting if you want loop failure to
exit the terminal application automatically, as the example does. Normal
shutdown should execute both Close commands **before** quitting Tea so that
terminal placements are removed as well as local resources released.

With zero-value options, `Init` starts `RunGame` in a background goroutine.
That convenience path is available for environments known to tolerate it; it
does not satisfy Ebitengine's documented main-thread contract and may fail or
crash, especially on macOS. Do not try it first and retry with `Run`: Ebitengine
cannot restart. `Run` requires `ManualRun` so startup cannot race the default
background launcher.

## Rendering and pacing

- Each request captures `RasterGeometry()` and a generation. One reusable
  `*ebiten.Image` is cleared before drawing and recreated only when `Pixels`
  changes. The hidden window's own screen is an unused 1×1 logical surface.
- `DrawFunc` runs on the Ebitengine callback goroutine. Do not access the Picture
  or Tea model from it. Use immutable snapshots or synchronization for shared
  application state. Do not retain, dispose, or resize `dst` or retain its pixels.
- `ReadPixels` performs GPU→CPU readback into a **new `*image.RGBA`** for each
  frame. These bytes are **premultiplied RGBA**, exactly as Go's `image.RGBA`
  expects; do not reinterpret them as straight-alpha `image.NRGBA`. Opaque
  frames with `FitFill` avoid additional scaling/compositing in ntpicture.
- Frames are submitted on the Tea event loop with
  `SetImageImmutableFor(img, geo)`. Their CPU memory is never reused or mutated.
  `StableID` lets Kitty replace pixels under the same image ID while geometry
  stays fixed, avoiding placeholder churn. ntpicture handles dirty rectangles
  and unchanged frames when the terminal confirms frame-edit support, and
  synchronizes full uploads to avoid flashes during same-ID replacement. See
  its [Kitty frame-update guidance](https://github.com/NimbleTerminal/ntpicture/blob/main/KITTY.md#frame-updates).
- At most one render is in flight. After submission, the producer waits for an
  owned `PresentedMsg` or `StaleFrameMsg`. An old or foreign presentation cannot
  release the current render slot. `PresentedMsg` means ntpicture committed
  ordered output, not that the terminal acknowledged display.
- Size, mode, cell metrics, display aspect, and resolution changes invalidate
  the current generation. A canceled callback still occupies the slot until it
  returns; its result is dropped, then the latest geometry is rendered. Zero
  pixel dimensions suspend production. Viewport changes do not invalidate the
  raster or request extra frames.
- `TPS` defaults to 60 and controls request pickup in Ebitengine's `Update`.
  There is no independent animation timer or frame backlog; presentation and
  readback costs determine throughput. Use elapsed time in your drawing code
  for animation independent of delivered frame rate.
- Draw and Picture errors emit an owned `ErrorMsg`, appear in `Status().Err`,
  and pause production until geometry changes. Loop exit is terminal. `Close`
  cancels requests, discards late results, and waits for `RunGame` to exit via
  `ebiten.Termination`. A callback already running cannot be forcibly interrupted.

See ntpicture's [live-framebuffer documentation](https://github.com/NimbleTerminal/ntpicture#framebuffers-and-exact-pixels)
and [framebuffer producer example](https://github.com/NimbleTerminal/ntpicture/tree/main/examples/framebuffer)
for the presentation contract this adapter follows.

## Diagnostics

Read `producer.Status()` on the Bubble Tea event loop. `LastFrame` describes
the most recent producer submission confirmed by an exact matching
`PresentedMsg`; `LastFrame.Generation == 0` means no sample yet.

```go
status := producer.Status()
frame := status.LastFrame
// frame.DrawTime: inside DrawFunc (CPU calls and any blocking work)
// frame.ReadbackTime: inside ReadPixels (GPU synchronization and copying)
// frame.PresentTime: submission through PresentedMsg delivery to Update
// frame.Geometry, frame.Bytes: that frame's raster and raw RGBA buffer size
// status.PresentedFrames: exact confirmations of producer submissions
// status.DiscardedRenders: obsolete results drained after geometry changes
```

These are wall-clock stages, not GPU execution timings or terminal display
acknowledgements. The byte count is not compressed transport traffic. The last
sample remains available across resize, errors, and closure; its geometry may
differ from the current `Status().Geometry`. Diagnostics do not change pacing.

Run `go run ./examples/loops` for live measurements. See the
[diagnostics guide](docs/hugo/content/guide/diagnostics.md) for counter semantics,
measurement boundaries, and experiments to identify expensive stages.

## Constraints

- One producer / Ebitengine loop per process, with no restart after closure.
  This adapter cannot coexist with another independently running `RunGame`.
  It owns global Ebitengine TPS and window-visibility settings.
- The window is hidden by default using `SetWindowVisible(false)` before
  `RunGame`. Set `HideWindow` to a pointer to `false` for diagnostics; the visible
  window does not mirror the terminal framebuffer.
- Hidden does not mean software-rendered or display-server-free. Ebitengine
  still needs a graphics environment. The [software renderer request (#353)](https://github.com/hajimehoshi/ebiten/issues/353)
  is separate; `exp/vmhost` is not used here.
- [Ebitengine 2.10 desktops do not require Cgo](https://ebitengine.org/en/documents/2.10.html),
  but platform runtime libraries/drivers are still required. This adapter targets
  desktop terminal applications and the browser setup demonstrated in `web/`;
  mobile loop integration is not provided.
- Readback is a synchronization and copying cost; this is not a zero-copy GPU
  presentation path. Lower the picture's Kitty resolution or raster pixel cap
  for expensive scenes. Oversized GPU surfaces are reported as drawing errors.
- The host must keep forwarding messages, and must not submit competing images
  to this picture. Close the producer before closing or keeping the picture.

## Checks

```sh
go test ./...
go test -race ./...
go vet ./...
# Optional real GPU readback test; needs a desktop graphics environment:
go test -tags integration .
```

The default tests exercise pacing, geometry invalidation, error recovery, owner
filtering, and cancellation without starting `RunGame`. The integration test
runs a hidden loop on the main thread and checks exact sizes, surface clearing,
premultiplied pixels, immutable frame ownership, and callback errors.

[Test CI](.github/workflows/test.yml) runs on pushes and pull requests using the
Go version in `go.mod`. It runs vet, the full suite with the race detector, and
the integration suite with the race detector. On Linux, Xvfb supplies an X server
and Mesa supplies software OpenGL, so the graphics tests exercise the hidden
loop and readback without a physical GPU. This does not measure hardware GPU
performance or verify every platform's graphics driver.

### ntui overlay lab

`go run ./examples/ntui` adds floating immediate-mode controls to the shared
shader lab: effect, palette, strength, pause, editable text, light/dark panels,
gauges, and presented-FPS plots. Drag either title bar and scroll inside panels.
The [browser gallery](https://nimbleterminal.dev/ntebitengine/?demo=ntui) includes
the same demo. The backend is included in the required ntui v0.2.0 release.
See [its README](examples/ntui/README.md) for controls and the
[event-loop guide](docs/hugo/content/guide/event-loops.md#ntui-overlays).
