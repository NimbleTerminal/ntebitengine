package shaderlab

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

// Ordinary Go builds embed Kage verbatim; compile it explicitly so language
// errors are caught without launching a window or waiting for the first frame.
func TestShadersCompile(t *testing.T) {
	for name, source := range map[string][]byte{"field": fieldSource, "CRT": crtSource} {
		t.Run(name, func(t *testing.T) {
			shader, err := ebiten.NewShader(source)
			if err != nil {
				t.Fatal(err)
			}
			shader.Deallocate()
		})
	}
}
