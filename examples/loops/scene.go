package main

import (
	"image/color"
	"math"
	"sync/atomic"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"nimbleterminal.dev/ntpicture"
)

// snapshot contains values only. Bubble Tea owns the simulation and publishes
// a complete copy; a callback loads exactly one copy at its start.
type snapshot struct {
	phase    float64
	revision uint64
	slow     bool
}

type drawReport struct {
	revision uint64
	calls    uint64
	duration time.Duration // callback only; excludes readback and presentation
}

type scene struct {
	input  atomic.Value // snapshot: Tea writes, Ebitengine reads
	report atomic.Value // drawReport: Ebitengine writes, Tea reads
	calls  uint64       // Ebitengine callback owns this counter
}

func newScene() *scene {
	s := &scene{}
	s.input.Store(snapshot{})
	s.report.Store(drawReport{})
	return s
}

func (s *scene) draw(dst *ebiten.Image, geo ntpicture.RasterGeometry) error {
	started := time.Now()
	state := s.input.Load().(snapshot)
	if state.slow {
		// Deliberate demonstration of a busy callback, NOT a rendering recipe.
		// Bubble Tea keeps running. Cancellation must wait for this to return.
		time.Sleep(80 * time.Millisecond)
	}
	dst.Fill(color.RGBA{R: 15, G: 24, B: 39, A: 255})
	const height = 100.0
	width := height * geo.DisplayAspect
	sx, sy := float64(geo.Pixels.X)/width, float64(geo.Pixels.Y)/height
	line := func(x1, y1, x2, y2, weight float64, c color.Color) {
		vector.StrokeLine(dst, float32(x1*sx), float32(y1*sy), float32(x2*sx), float32(y2*sy), float32(weight*sy), c, true)
	}
	// Tick marks and an orbit use physical coordinates, including glyph mode.
	radius := math.Min(31, width*0.35)
	for i := 0; i < 60; i++ {
		a := float64(i) * 2 * math.Pi / 60
		inner := radius * 0.94
		if i%5 == 0 {
			inner = radius * 0.85
		}
		line(width/2+inner*math.Cos(a), 50+inner*math.Sin(a), width/2+radius*math.Cos(a), 50+radius*math.Sin(a), 0.45, color.RGBA{R: 78, G: 105, B: 131, A: 255})
	}
	for i := 12; i >= 0; i-- {
		a := state.phase - float64(i)*0.09
		x, y := width/2+radius*math.Cos(a), 50+radius*math.Sin(a)
		v := uint8(255 - i*16)
		var path vector.Path
		path.MoveTo(float32((x+2)*sx), float32(y*sy))
		for j := 1; j <= 24; j++ {
			angle := float64(j) * 2 * math.Pi / 24
			path.LineTo(float32((x+2*math.Cos(angle))*sx), float32((y+2*math.Sin(angle))*sy))
		}
		path.Close()
		op := &vector.DrawPathOptions{AntiAlias: true}
		op.ColorScale.ScaleWithColor(color.RGBA{R: v / 3, G: v, B: uint8(uint16(v) * 3 / 4), A: 255})
		vector.FillPath(dst, &path, nil, op)
	}
	line(width/2, 50, width/2+radius*math.Cos(state.phase), 50+radius*math.Sin(state.phase), 0.7, color.RGBA{R: 251, G: 191, B: 112, A: 255})
	s.calls++
	s.report.Store(drawReport{revision: state.revision, calls: s.calls, duration: time.Since(started)})
	return nil
}
