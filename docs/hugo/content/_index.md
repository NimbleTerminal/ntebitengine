---
title: "ntebitengine"
bookToC: false
---

# ntebitengine

Draw with Ebitengine, then present the result in a Bubble Tea terminal through
ntpicture. The adapter owns the hidden graphics loop, reads back each requested
frame, and paces production against terminal presentation.

Start with [Managing the event loops](guide/event-loops/): where each loop runs,
who owns state, how frames cross the boundary, and how both loops shut down.

The [Loop lab](guide/loop-lab/) makes that boundary visible with independent
simulation and drawing counters. Slow down the drawing callback and keep
interacting with the terminal to see the handoff in action.

[Measuring frame costs](guide/diagnostics/) explains the producer's timing and
frame diagnostics, also displayed in the Loop lab.

- [Try all demos](../)
- [Source and package README](https://github.com/NimbleTerminal/ntebitengine)
- [API reference](https://pkg.go.dev/nimbleterminal.dev/ntebitengine)
