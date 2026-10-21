//go:build integration

package ntebitengine

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"os"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"nimbleterminal.dev/ntpicture"
)

var gpuLoop *gameLoop
var gpuDrawError = errors.New("GPU callback error")

// The optional integration suite owns one real hidden loop on the main thread.
// Run with a working desktop graphics environment: go test -tags integration .
func TestMain(m *testing.M) {
	gpuLoop = newGameLoop(func(dst *ebiten.Image, geo ntpicture.RasterGeometry) error {
		if dst.Bounds().Size() != geo.Pixels {
			return errors.New("wrong GPU target dimensions")
		}
		switch geo.Cells.X {
		case 1:
			dst.Fill(color.RGBA{R: 64, G: 32, B: 16, A: 128})
		case 2: // Leave blank to verify the reusable surface was cleared.
		case 3:
			dst.Fill(color.RGBA{R: 255, A: 255})
		case 4:
			return gpuDrawError
		}
		return nil
	}, Options{})
	result := make(chan int, 1)
	go func() {
		code := m.Run()
		gpuLoop.close()
		result <- code
	}()
	err := gpuLoop.run()
	code := <-result
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		code = 1
	}
	os.Exit(code)
}

func TestGPUReadback(t *testing.T) {
	frames := []struct {
		size image.Point
		want color.RGBA
	}{
		{image.Pt(13, 7), color.RGBA{R: 64, G: 32, B: 16, A: 128}},
		{image.Pt(13, 7), color.RGBA{}},
		{image.Pt(9, 11), color.RGBA{R: 255, A: 255}},
	}
	var previous *image.RGBA
	for i, frame := range frames {
		r := &request{ctx: context.Background(), geometry: ntpicture.RasterGeometry{Cells: image.Pt(i+1, 1), Pixels: frame.size}, reply: make(chan renderResult, 1)}
		gpuLoop.requests <- r
		got := receive(t, r.reply)
		if got.err != nil {
			t.Fatal(got.err)
		}
		if got.img.Bounds().Size() != frame.size {
			t.Fatal("wrong CPU dimensions")
		}
		for y := 0; y < frame.size.Y; y++ {
			for x := 0; x < frame.size.X; x++ {
				if pixel := got.img.RGBAAt(x, y); pixel != frame.want {
					t.Fatalf("frame %d pixel (%d,%d) = %v, want %v", i, x, y, pixel, frame.want)
				}
			}
		}
		if i == 0 {
			previous = got.img
		}
		if previous.RGBAAt(0, 0) != frames[0].want {
			t.Fatal("previous immutable frame was overwritten")
		}
	}
	r := &request{ctx: context.Background(), geometry: ntpicture.RasterGeometry{Cells: image.Pt(4, 1), Pixels: image.Pt(2, 2)}, reply: make(chan renderResult, 1)}
	gpuLoop.requests <- r
	if got := receive(t, r.reply); got.err != gpuDrawError || got.img != nil {
		t.Fatal("draw error did not suppress readback")
	}
}
