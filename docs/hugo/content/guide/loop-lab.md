---
title: "Loop lab"
weight: 20
---

# Loop lab

[Run it in the browser](../../../?demo=loops), or from the repository:

```sh
go -C examples run ./loops
```

Read [Managing the event loops](../event-loops/) alongside the
[example source](https://github.com/NimbleTerminal/ntebitengine/tree/main/examples/loops).

## Try these experiments

1. **Watch both counters.** Tea advances an orbit simulation on its own timer.
   Drawing samples the latest snapshot, and presentation follows ntpicture's
   pacing. The counters need not advance together.
2. **Press `l` for a slow callback.** Each draw now waits 80ms. Tea's tick and
   snapshot counters keep advancing. Change speed or pause: the controls still
   respond, while each completed draw reflects the snapshot it captured earlier.
3. **Press Space.** Simulation phase freezes. Ticks, drawing, and presentation
   continue; pausing the scene does not close either event loop.
4. **Resize, toggle `m`, or change resolution with `+`/`-`.** The producer changes
   generation and drops obsolete work. There is no queue of frames to drain
   before the new size appears, but a running callback must finish first.
5. **Quit while slow drawing is enabled.** Shutdown waits for that callback,
   then removes the picture and exits Tea. It does not restart Ebitengine.

## Read the diagnostics

| Row | Meaning |
| --- | --- |
| TEA | Handled simulation ticks, latest published revision, simulation phase |
| DRAW | Completed callbacks, revision sampled by the most recent callback, callback duration |
| PIPE | Producer state, request generation, submitted frames, all owned picture commits |
| TIME | Draw callback, readback, and presentation duration for one confirmed producer frame |
| FRAME | That frame's raster size, raw RGBA bytes, producer generation, confirmed count, obsolete-render count |

The callback duration includes the artificial sleep and CPU draw calls. It
excludes the adapter's subsequent readback and ntpicture's preparation/output;
it is **not GPU execution time**. Callback reports can describe frames later
discarded on resize. Request generations and snapshot revisions are different
counters and should not be compared for equality.

The presented count includes owned Picture commits caused by geometry changes
as well as newly submitted frames. It need not equal the submitted count.

The TIME and FRAME rows update together only after an exact matching producer
submission is confirmed. They can lag behind the DRAW row and the current
geometry, especially with slow rendering or during a resize. Read
[Measuring frame costs](../diagnostics/) for the precise measurement boundaries.

## Controls

| Key | Action |
| --- | --- |
| Space | Pause/resume simulation |
| `l` | Toggle the deliberate 80ms callback delay |
| `[` / `]` | Decrease/increase simulation speed |
| `r` | Reset simulation phase |
| `m` | Toggle glyph/Kitty mode |
| `+` / `-` | Change Kitty raster resolution |
| `q`, Esc, Ctrl+C | Close both loops and quit |

The example caps elapsed simulation time at 100ms after stalls. It is a visual
ownership demonstration, not a fixed-step physics engine. Browser tabs can be
throttled when backgrounded; a hidden desktop window still needs a working GPU
and graphics environment.
