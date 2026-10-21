package main

import (
	"github.com/hajimehoshi/ebiten/v2"
	"nimbleterminal.dev/ntebitengine/examples/internal/shaderlab"
	"nimbleterminal.dev/ntpicture"
	"sync/atomic"
	"time"
)

var effectNames = shaderlab.EffectNames
var palettes = shaderlab.PaletteNames

type controls struct {
	effect, palette int
	strength        float64
	paused          bool
}
type scene struct {
	controls atomic.Value
	renderer shaderlab.Scene
	last     time.Time
	elapsed  float64
}

func newScene(c controls) *scene { s := &scene{}; s.controls.Store(c); return s }
func (s *scene) draw(dst *ebiten.Image, geo ntpicture.RasterGeometry) error {
	c := s.controls.Load().(controls)
	now := time.Now()
	if !s.last.IsZero() && !c.paused {
		s.elapsed += now.Sub(s.last).Seconds()
	}
	s.last = now
	return s.renderer.Draw(dst, geo, shaderlab.Controls{Effect: c.effect, Palette: c.palette, Strength: c.strength, Paused: c.paused}, s.elapsed)
}
