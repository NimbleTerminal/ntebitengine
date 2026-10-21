package shaderlab

import (
	_ "embed"
	"fmt"

	"github.com/hajimehoshi/ebiten/v2"
	"nimbleterminal.dev/ntpicture"
)

//go:embed field.kage
var fieldSource []byte

//go:embed crt.kage
var crtSource []byte

var EffectNames = []string{"plasma", "rings", "CRT"}
var PaletteNames = []string{"violet / teal", "amber / rose", "blue / mint"}
var palettes = [...][3]float32{{0, .18, .35}, {0, .08, .18}, {.30, .18, 0}}

type Controls struct {
	Effect, Palette int
	Strength        float64
	Paused          bool
}

// Scene's GPU resources belong exclusively to the Ebitengine callback.
type Scene struct {
	field, crt *ebiten.Shader
	source     *ebiten.Image
}

func (s *Scene) Draw(dst *ebiten.Image, geo ntpicture.RasterGeometry, c Controls, elapsed float64) error {
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
	s.render(dst, geo, c, elapsed)
	return nil
}

func (s *Scene) render(dst *ebiten.Image, geo ntpicture.RasterGeometry, c Controls, elapsed float64) {
	mode := float32(0)
	if c.Effect == 1 {
		mode = 1
	}
	palette := palettes[c.Palette]
	op := &ebiten.DrawRectShaderOptions{Uniforms: map[string]any{
		"Time": float32(elapsed), "Aspect": float32(geo.DisplayAspect),
		"Strength": float32(c.Strength), "Mode": mode, "Palette": palette[:],
	}}
	w, h := geo.Pixels.X, geo.Pixels.Y
	if c.Effect != 2 {
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
		Uniforms: map[string]any{"Time": float32(elapsed), "Strength": float32(c.Strength)},
	})
}
