//go:build js && wasm

package demorun

import (
	tea "charm.land/bubbletea/v2"
	"github.com/hajimehoshi/ebiten/v2"
	booba "nimbleterminal.dev/booba"
)

func init() {
	// The terminal owns keyboard focus; Ebitengine's canvas is only a GPU host.
	ebiten.SetInitFocused(false)
}

func Run(model tea.Model) error { return booba.Run(model) }
