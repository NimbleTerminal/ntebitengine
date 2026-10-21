# Loop lab

```sh
go run ./examples/loops
```

This example accompanies the [event-loop guide](../../docs/hugo/content/guide/event-loops.md).
Bubble Tea owns an orbit simulation and publishes immutable snapshots. The
Ebitengine callback reads one snapshot per frame and returns a separate atomic
diagnostic report. The producer owns all frame scheduling and readback.

Press `l` to add an intentional 80ms delay inside the callback. Input and Tea's
simulation keep running; rendering samples the latest available state without
building a backlog. Press Space to freeze simulation while both loops continue.
Use `[`/`]` for speed, `r` to reset, `m` for glyph/Kitty, `+`/`-` for Kitty
resolution, and `q` to close both loops.

The top rows distinguish Tea ticks and snapshot revisions, completed draw
callbacks, and producer submissions/presentations. Callback timing excludes GPU
readback and terminal output. The TIME and FRAME rows use the producer's
`Status().LastFrame` to show draw, readback, and submission-to-presentation
timings for the same confirmed frame, alongside its raster size and raw RGBA
bytes. Obsolete renders count results discarded after geometry changes.
Read the [experiments and counter definitions](../../docs/hugo/content/guide/loop-lab.md)
and [diagnostics guide](../../docs/hugo/content/guide/diagnostics.md).

The same demo appears on the browser page at `?demo=loops`. The deliberate delay
is for demonstration only; production draw callbacks should return promptly.
