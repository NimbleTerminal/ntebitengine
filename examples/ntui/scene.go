package main

import (
	"sync/atomic"

	"github.com/hajimehoshi/ebiten/v2"
	"nimbleterminal.dev/ntebitengine/examples/internal/shaderlab"
	"nimbleterminal.dev/ntpicture"
	"nimbleterminal.dev/ntui"
	ntuiebiten "nimbleterminal.dev/ntui/ebiten"
)

// One publication couples widget output with the scene state it describes.
// Commands and their Values are immutable after Store.
type snapshot struct {
	controls shaderlab.Controls
	phase    float64
	commands []ntui.DrawCmd
	geometry ntpicture.RasterGeometry
	glyph    bool
}
type scene struct {
	input    atomic.Value
	renderer shaderlab.Scene
	overlay  *ntuiebiten.Backend // created and used only in DrawFunc
}

func newScene() *scene { s := &scene{}; s.input.Store(snapshot{}); return s }
func (s *scene) draw(dst *ebiten.Image, geo ntpicture.RasterGeometry) error {
	state := s.input.Load().(snapshot) // exactly one load for scene and UI
	if err := s.renderer.Draw(dst, geo, state.controls, state.phase); err != nil {
		return err
	}
	// A request can precede a resize publication. Never draw UI laid out for a
	// different geometry. Producer already invalidates the obsolete final frame.
	if state.glyph || state.geometry != geo || geo.Cells.X <= 0 || geo.Cells.Y <= 0 {
		return nil
	}
	if s.overlay == nil {
		s.overlay = ntuiebiten.New()
	}
	s.overlay.Render(dst, state.commands, ntuiebiten.RenderOptions{
		ScaleX: float64(geo.Pixels.X) / float64(geo.Cells.X),
		ScaleY: float64(geo.Pixels.Y) / float64(geo.Cells.Y),
	})
	return nil
}
