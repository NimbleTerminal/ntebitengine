---
title: "Measuring frame costs"
weight: 30
---

# Measuring frame costs

FPS describes throughput. To see where a frame spends time, read
`producer.Status().LastFrame` from Bubble Tea's event loop. The producer measures
the callback, readback, and presentation stages without adding a render queue or
changing presentation pacing.

All three durations belong to **one frame confirmed by an exact matching
`PresentedMsg`**. A sample is published only after that message reaches
`Producer.Update`. Failed, stale, duplicate, foreign, or superseded confirmations
do not create a sample.

```go
func frameTiming(producer *ntebitengine.Producer) string {
    frame := producer.Status().LastFrame
    if frame.Generation == 0 {
        return "Waiting for a confirmed frame"
    }
    return fmt.Sprintf("draw %s | readback %s | present %s",
        frame.DrawTime, frame.ReadbackTime, frame.PresentTime)
}
```

`Status` is a value snapshot. Call it on Tea's event loop; do not poll the producer
from a telemetry goroutine or the Ebitengine callback. Copy the returned value
across any additional concurrency boundary.

## Timing boundaries

| Field | Measures | Does not mean |
| --- | --- | --- |
| `DrawTime` | Wall time spent inside your `DrawFunc` | GPU execution time |
| `ReadbackTime` | Wall time inside `ReadPixels`, including GPU synchronization and copying | Pure transfer bandwidth |
| `PresentTime` | Time immediately before `SetImageImmutableFor` through delivery of its matching `PresentedMsg` to `Producer.Update` | Time until the monitor displays the frame |

Drawing can enqueue GPU work. Consequently, some cost caused by a complex scene
can appear in readback when the GPU must finish that work. The callback time also
includes font/shader initialization, CPU work, sleeps, and scheduler delays that
occur inside the callback.

Presentation time includes synchronous submission work, command scheduling,
ntpicture preparation/encoding, committing output, and message delivery. It is
not a measurement of terminal transfer alone. An application that forwards the
message late will report a longer presentation time.

The stages exclude request pickup, adapter-owned surface allocation and clearing,
CPU framebuffer allocation, and delivery of the render result back to Tea. Do not
sum them and label the result total frame latency or derive a promised FPS from
their reciprocal. Small values may round to zero on a platform's clock.

## Frame identity and size

`LastFrame.Generation` is the producer request generation, distinct from a
Picture generation. Zero means no confirmed sample exists yet.

`LastFrame.Geometry` is the geometry of that measured frame, while
`Status().Geometry` describes the producer's current target. During resize,
errors, or shutdown the last sample remains available and may describe an older
target. A newer pending submission does not erase it.

`LastFrame.Bytes` counts the immutable CPU RGBA buffer: four bytes per raster
pixel. This is neither compressed Kitty traffic nor glyph output size. It is
also not total memory consumption; GPU surfaces, preparation buffers, and other
retained images have their own costs.

## Counters

Counters are cumulative for the producer's lifetime:

- **`Frames`** counts submissions to Picture, as before.
- **`PresentedFrames`** counts exact matching confirmations of producer
  submissions, once each. It excludes Picture-only redraws and confirmations
  superseded by newer Picture generations. It is a conservative confirmed count,
  not a record of every image that physically appeared on screen.
- **`DiscardedRenders`** counts obsolete render results drained after geometry
  changes. A burst of ten resize messages can invalidate one request and produce
  just one discard. A request canceled before its callback or readback also
  counts when drained. Discarded submitted Picture jobs, ordinary errors, and
  shutdown cancellations are not included.

These counters do not form a complete accounting equation. There may be work
in flight, an unconfirmed submission, a failed render, or a Picture-only commit.

## Try the Loop lab

```sh
go -C examples run ./loops
```

Its TIME and FRAME rows show the producer sample. The DRAW row independently
reports the latest callback completion, which may be newer or eventually
discarded. The header's FPS counts owned Picture commits; it can differ from a
rate calculated using `PresentedFrames` deltas.

1. **Press `l` to enable the 80ms callback delay.** After a frame completes,
   `DrawTime` should reflect at least that deliberate wait. The UI keeps updating
   its independent simulation ticks. This is not a GPU benchmark.
2. **Compare Kitty resolutions with `+`/`-`.** Watch pixel dimensions and raw
   byte size, then compare readback and presentation costs over several frames.
   Halving both raster dimensions approximately quarters raw bytes; actual time
   depends on the scene, graphics backend, encoding, and terminal.
3. **Toggle glyph mode with `m`.** The raw framebuffer becomes small, but glyph
   encoding and output can still dominate. Raw RGBA bytes do not predict wire
   traffic across the two presentation modes.
4. **Resize during slow drawing.** The obsolete-render count should increase
   when invalidated work returns. The last confirmed sample stays visible until
   a replacement is confirmed. Resizing without a render in flight need not
   increase that count.

Use repeated observations after warm-up to decide what to profile next. High
draw time suggests inspecting callback work. High readback time can indicate
GPU work or synchronization/transfer cost. High presentation time points toward
preparation, output, or event-loop scheduling. The sample is the latest frame,
not a rolling average or percentile, and these signals are starting points for
profiling rather than proof of a specific bottleneck.
