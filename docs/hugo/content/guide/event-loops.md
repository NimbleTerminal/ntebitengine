---
title: "Managing the event loops"
weight: 10
---

# Managing the Bubble Tea and Ebitengine event loops

Give each loop a clear owner. Bubble Tea owns input, application state, layout,
the Picture, and the Producer. Ebitengine owns GPU resources and drawing. Pass
immutable snapshots between them, and let the producer schedule frames when
ntpicture is ready to present another one.

The [Loop lab](../loop-lab/) is a complete runnable example of this arrangement.
Its simulation advances in Bubble Tea; the GPU callback draws the latest state
without advancing that simulation itself.

## What runs where

| Work | Owner |
| --- | --- |
| Terminal keys, mouse, layout, simulation in this demo | Bubble Tea event loop |
| `pic.Init/Update/View`, geometry setters, producer methods except `Run` | Bubble Tea event loop |
| Waiting for work, asynchronous I/O | `tea.Cmd` functions; return messages instead of mutating the model |
| `producer.Run()` | Main goroutine; occupies the main OS thread as required by Ebitengine |
| `DrawFunc`, GPU images, shaders, font caches | Ebitengine callbacks |
| Pixel readback and frame handoff | Adapter |

There is one Ebitengine loop per process. Do not create a second `RunGame` to
animate a scene or open a second producer; compose the scene into the existing
destination. Closing that loop ends its lifetime: it cannot be restarted.

The name `DrawFunc` describes your work, not the Ebitengine callback it runs
inside. The adapter invokes it from its internal **`Game.Update`**. Its
`Game.Draw` is empty, and `Layout` returns a tiny unused screen. The offscreen
destination is independently sized to the picture's exact raster geometry.

## Start both loops

Construct the model with `ManualRun: true`. Start Bubble Tea in a goroutine,
then call `Producer.Run` directly from `main`'s call stack:

```go
func run() error {
    m := newModel() // New(..., Options{ManualRun: true})
    done := make(chan error, 1)
    go func() {
        _, err := tea.NewProgram(m).Run()
        // Also handles startup failure or a signal exit before normal cleanup.
        m.producer.Close()()
        _ = m.pic.Close()
        done <- errors.Join(err, m.err)
    }()
    loopErr := m.producer.Run()
    return errors.Join(loopErr, <-done)
}
```

The cleanup after `Program.Run` happens after Tea has stopped accessing the
model. `Producer.Run` only accesses synchronized loop state; it does not read
the Tea model. `Run` may reach the loop before or after `Init`. If Tea fails
before initialization, `Close` still lets `Run` finish.

The example's `demorun.Run(m)` selects `tea.NewProgram(m).Run()` on desktops and
the booba terminal bridge in WebAssembly. The ownership pattern stays the same.

Separate goroutines do not guarantee parallel CPU execution. In a browser,
long synchronous work or GPU readback can still stall the main browser thread.
The demo's sleep yields to the Go scheduler; it demonstrates the handoff
protocol, not a promise that arbitrary expensive rendering leaves input smooth.

Zero-value `Options` start Ebitengine from a background goroutine. That is a
convenience for environments known to support it, but it does not satisfy
[Ebitengine's main-thread contract](https://pkg.go.dev/github.com/hajimehoshi/ebiten/v2#RunGame).
Use `ManualRun` for a portable desktop application. Do not try background startup
and then retry on the main thread after failure: the loop cannot restart.

## Initialize and route messages

Create the Picture with `StableID: true`, `AutoMode: true`, and `FitFill`. The
host owns Picture initialization and layout; the producer is its sole image
source. Leave `ClearOnReplace` disabled for live frames.

```go
func (m *model) Init() tea.Cmd {
    return tea.Batch(m.pic.Init(), m.producer.Init(), tick())
}
```

For every normal update:

1. Handle input and apply layout changes such as `pic.SetSize`.
2. Publish changed application state for the renderer.
3. Call `pic.Update(msg)` so capability discovery can update raster geometry.
4. Call `producer.Update(msg)` and execute every returned command.

```go
// At the end of Update, after host input/layout handling:
cmds = append(cmds, m.pic.Update(msg))
cmds = append(cmds, m.producer.Update(msg))
return m, tea.Batch(cmds...)
```

Call the methods on the event loop. Do not wrap model mutations in commands or
start goroutines that call Picture setters. `tea.Batch` allows its commands to
run concurrently; their completion order is not an ordering guarantee. Use
`tea.Sequence` when command completion must be ordered, as during shutdown.

`View` only builds the terminal view. Do not render, submit images, advance the
simulation, or initialize GPU assets there.

## Share snapshots, not a mutable model

In the demo, Tea owns a small value struct and publishes a copy through
`atomic.Value`. Initialize the value before either loop starts:

```go
type snapshot struct {
    phase    float64
    revision uint64
    slow     bool
}

// Setup:
s.input.Store(snapshot{})

// Bubble Tea's event loop:
m.state.revision++
m.scene.input.Store(m.state)

// Ebitengine callback, once at the start of the frame:
state := s.input.Load().(snapshot)
```

A slow frame keeps drawing the snapshot it loaded. Input can publish newer
snapshots meanwhile; the next callback reads the latest one. Intermediate
simulation states can be skipped without allocating a queue of obsolete frames.

Copying a struct containing slices, maps, or pointers does **not** copy their
contents. Clone those contents before publication, or use a mutex and copy the
necessary data while holding it briefly. Do not hold a shared lock through a
draw or readback. All stores in one `atomic.Value` must have the same concrete
type, and it must not be copied after use.

Snapshots represent state. An action that must happen exactly once—saving a
file or applying a transaction, for example—needs a message/command protocol,
not a boolean that a later snapshot can overwrite. Keep any such queue bounded
and define overflow and shutdown behavior.

The reverse direction follows the same rule. The callback stores a small
`drawReport`; Tea reads it without accessing callback-owned fields. Reports may
be skipped: these are diagnostics, not an acknowledgement protocol. Never send
a blocking message back to Tea from a callback while Tea is waiting for the
loop to close.

## Understand the three clocks

There are separate rates for application updates, Ebitengine request pickup,
and terminal presentation:

- **Simulation:** the demo schedules a one-shot `tea.Tick` about every 1/30
  second and schedules the next only after handling it. This is a target cadence,
  not a real-time guarantee. It advances using elapsed time, capped at 100ms
  after a long stall, and freezes its phase while paused.
- **Ebitengine TPS:** defaults to 60. It controls how soon the adapter can pick
  up pending work; it does not promise 60 draws or 60 presentations per second.
- **Presentation:** a new request normally starts only after the previous
  frame's owned `PresentedMsg` or `StaleFrameMsg` releases the slot.

Do not call `SetImageImmutableFor` or request renders from your simulation tick;
the producer already owns that path. A fast simulation can publish several
states between frames. A faster renderer can draw the same simulation state
more than once. Both are expected.

For deterministic physics, use a fixed-step accumulator with a bounded catch-up
policy instead of this demo's variable elapsed-time step. Choose what a long
stall means for the application rather than allowing an unlimited catch-up loop
to freeze input. Purely visual animation can instead own elapsed time in the
drawing callback, as the sprite and shader demos do. Choose one owner for each
piece of state; do not advance it in both loops.

Keyboard input belongs to Bubble Tea. Polling `ebiten.IsKeyPressed` would read
input from Ebitengine's window/canvas, which is hidden here and does not own
terminal focus.

## Follow a frame across the boundary

```text
Tea model ── immutable snapshot ───────────────────┐
                                                ▼
Producer request (geometry + generation) → Ebitengine Update
                                                │
                                     clear → DrawFunc → ReadPixels
                                                │
Tea receives result ← fresh immutable image.RGBA ─┘
        │
        ├─ obsolete geometry/generation? discard
        ▼
Picture.SetImageImmutableFor → prepare/commit → PresentedMsg
                                                │
                                       next producer request
```

There is at most one render in flight. The producer waits for ntpicture to
commit ordered output before requesting the next frame. `PresentedMsg` is not
a hardware display acknowledgement. `Status().Frames` counts submissions, so
it is not the presented-frame counter either.

The adapter owns `dst`: it is already cleared and sized to `geo.Pixels`. Do not
retain, resize, dispose, or send it to Tea. Create reusable scene GPU resources
inside the callback and keep them on that side of the boundary. Readback creates
a fresh `image.RGBA` with premultiplied pixels; do not mutate that frame after
submission.

## Resizing and cancellation

Use the `geo` argument for that frame's size and physical aspect. Do not call
`pic.RasterGeometry()` from the callback: Picture belongs to Tea, and the current
geometry may already differ from the request you are rendering.

Size, mode, resolution, and cell-metric changes invalidate obsolete work. The
producer cancels the request, waits for any executing callback to return, drops
its late result, then renders the latest geometry. A zero-sized picture suspends
new rendering. Viewport clipping alone does not change raster geometry and does
not trigger an extra render.

Cancellation cannot interrupt `DrawFunc`. It has no context parameter. Keep it
short; do file/network loading in a cancellable Tea command, publish the decoded
data, and create the GPU resource in the callback. The demo's optional 80ms sleep
is intentionally bad rendering work used to expose this boundary. Turning it
off cannot change a callback that already captured the slow snapshot.

## Errors and shutdown

An owned `ntebitengine.ErrorMsg` reports a drawing, picture, or loop failure.
The demo records it and quits cleanly:

```go
case ntebitengine.ErrorMsg:
    if msg.Owner == m.producer {
        m.err = msg.Err
        return m, tea.Sequence(m.producer.Close(), m.pic.Close(), tea.Quit)
    }
```

Normal quit uses the same sequence. Calling `producer.Close()` immediately
cancels/stops the producer; executing its returned command waits for the loop
to finish. That wait belongs in a command, not synchronously inside Tea's
`Update`. Picture's cleanup command then removes terminal placements before
Tea exits. A running callback must return before the wait can finish.

Drawing and presentation failures pause production until geometry changes;
loop failure is terminal. An application that keeps running after an error
must account for that distinction. There is no automatic renderer fallback or
restart. `Close` is idempotent, which makes the post-`Program.Run` cleanup safe.

## Symptoms to check

| Symptom | Check |
| --- | --- |
| Startup crashes on a main-thread platform | Use `ManualRun` and call `Run` from main |
| First frame appears, then animation stops | Forward all messages and execute both components' returned commands |
| Input freezes during rendering | Look for drawing in Tea's `Update`, synchronous waits, or a shared lock held through GPU work |
| State races or objects tear between frames | Load one immutable snapshot; copy referenced slice/map contents |
| A resize briefly shows old work | Verify Picture updates precede Producer updates and only the producer submits images |
| Quit hangs | A callback or custom worker is not returning; cancellation is cooperative |
| TPS is high but presented FPS is low | Measure draw, readback, encoding, and terminal transfer separately |

Run `go test -race ./...` while developing ownership code, and exercise resize,
mode changes, slow callbacks, and quit in the [Loop lab](../loop-lab/).
