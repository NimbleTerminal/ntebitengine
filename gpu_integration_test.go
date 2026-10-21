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
	"time"

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
		case 5:
			time.Sleep(10 * time.Millisecond) // a known callback cost, separate from readback
			dst.Fill(color.White)
		}
		return nil
	}, Options{})
	result := make(chan int, 1)
	go func() {
		// Establish the graphics context and complete one readback before the
		// tests apply their shorter per-frame deadlines. Software drivers on CI
		// can take longer to initialize than an already-running renderer.
		started := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		warmup := &request{ctx: ctx, geometry: ntpicture.RasterGeometry{Pixels: image.Pt(1, 1)}, reply: make(chan renderResult, 1)}
		gpuLoop.requests <- warmup
		code := 1
		select {
		case frame := <-warmup.reply:
			if frame.err != nil {
				fmt.Fprintln(os.Stderr, "graphics startup:", frame.err)
			} else {
				fmt.Fprintln(os.Stderr, "graphics startup completed in", time.Since(started))
				code = m.Run()
			}
		case <-ctx.Done():
			fmt.Fprintln(os.Stderr, "graphics startup:", ctx.Err())
		}
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
		if got.drawTime < 0 || got.readbackTime <= 0 {
			t.Fatalf("missing or invalid GPU timings: draw=%v readback=%v", got.drawTime, got.readbackTime)
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
	if got := receive(t, r.reply); got.err != gpuDrawError || got.img != nil || got.readbackTime != 0 {
		t.Fatal("draw error did not suppress readback")
	}
}

func TestGPUCallbackTiming(t *testing.T) {
	r := &request{ctx: context.Background(), geometry: ntpicture.RasterGeometry{Cells: image.Pt(5, 1), Pixels: image.Pt(4, 4)}, reply: make(chan renderResult, 1)}
	gpuLoop.requests <- r
	got := receive(t, r.reply)
	if got.err != nil || got.img == nil {
		t.Fatalf("render failed: %v", got.err)
	}
	if got.drawTime < 10*time.Millisecond || got.readbackTime <= 0 {
		t.Fatalf("timings did not capture the callback: draw=%v readback=%v", got.drawTime, got.readbackTime)
	}
}
