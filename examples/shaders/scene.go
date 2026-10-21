package main

import (
	_ "embed"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"nimbleterminal.dev/ntpicture"
)

//go:embed field.kage
var fieldSource []byte

//go:embed crt.kage
var crtSource []byte

var effectNames = [...]string{"plasma", "rings", "CRT"}
var palettes = [...][3]float32{
	{0.00, 0.18, 0.35}, // violet, teal, and gold
	{0.00, 0.08, 0.18}, // warm amber and rose
	{0.30, 0.18, 0.00}, // blue and mint
}

type controls struct {
	effect, palette int
	strength        float64
	paused          bool
}

type scene struct {
	controls atomic.Value // immutable controls from the Bubble Tea event loop
	// Shader compilation, images, and animation state belong to DrawFunc.
	field, crt *ebiten.Shader
	source     *ebiten.Image
	last       time.Time
	elapsed    float64
}

func newScene(c controls) *scene {
	s := &scene{}
	s.controls.Store(c)
	return s
}

func (s *scene) draw(dst *ebiten.Image, geo ntpicture.RasterGeometry) error {
	if s.field == nil {
		field, err := ebiten.NewShader(fieldSource)
		if err != nil {
			return fmt.Errorf("compile field shader: %w", err)
		}
		crt, err := ebiten.NewShader(crtSource)
		if err != nil {
			field.Deallocate()
			return fmt.Errorf("compile CRT shader: %w", err)
		}
		s.field, s.crt = field, crt
	}
	c := s.controls.Load().(controls)
	now := time.Now()
	if !s.last.IsZero() && !c.paused {
		s.elapsed += now.Sub(s.last).Seconds()
	}
	s.last = now
	s.render(dst, geo, c, s.elapsed)
	return nil
}

func (s *scene) render(dst *ebiten.Image, geo ntpicture.RasterGeometry, c controls, elapsed float64) {
	mode := float32(0)
	if c.effect == 1 {
		mode = 1
	}
	palette := palettes[c.palette]
	op := &ebiten.DrawRectShaderOptions{Uniforms: map[string]any{
		"Time": float32(elapsed), "Aspect": float32(geo.DisplayAspect),
		"Strength": float32(c.strength), "Mode": mode, "Palette": palette[:],
	}}
	w, h := geo.Pixels.X, geo.Pixels.Y
	if c.effect != 2 {
		dst.DrawRectShader(w, h, s.field, op)
		return
	}
	// A source image must match the DrawRectShader rectangle. Keep it on the GPU,
	// recreating only on raster resize; only the final destination is read back.
	if s.source == nil || s.source.Bounds().Size() != geo.Pixels {
		if s.source != nil {
			s.source.Deallocate()
		}
		s.source = ebiten.NewImage(w, h)
	}
	s.source.Clear()
	s.source.DrawRectShader(w, h, s.field, op)
	dst.DrawRectShader(w, h, s.crt, &ebiten.DrawRectShaderOptions{
		Images:   [4]*ebiten.Image{s.source},
		Uniforms: map[string]any{"Time": float32(elapsed), "Strength": float32(c.strength)},
	})
}
