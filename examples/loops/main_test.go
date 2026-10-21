package main

import (
	"math"
	"testing"
	"time"
)

func TestSimulationPauseAndStall(t *testing.T) {
	now := time.Unix(0, 0)
	m := &model{scene: newScene(), speed: 1, lastTick: now}
	m.advance(now.Add(50 * time.Millisecond))
	if math.Abs(m.state.phase-0.05) > 1e-9 {
		t.Fatalf("phase = %g", m.state.phase)
	}
	m.paused = true
	m.advance(now.Add(10 * time.Second))
	if math.Abs(m.state.phase-0.05) > 1e-9 {
		t.Fatal("paused simulation advanced")
	}
	m.paused = false
	m.advance(now.Add(10*time.Second + 50*time.Millisecond))
	if math.Abs(m.state.phase-0.1) > 1e-9 {
		t.Fatal("resume caught up time spent paused")
	}
	m.advance(now.Add(30 * time.Second))
	if math.Abs(m.state.phase-0.2) > 1e-9 {
		t.Fatal("long stall was not capped to 100ms")
	}
	if m.scene.input.Load().(snapshot) != m.state {
		t.Fatal("latest simulation state was not published")
	}
}
