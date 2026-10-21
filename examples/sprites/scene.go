package main

import (
	"image"
	"image/color"
	"image/draw"
	"math"
	"sync/atomic"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"nimbleterminal.dev/ntpicture"
)

const spriteSize = 16

type controls struct {
	count  int
	paused bool
}

type scene struct {
	controls atomic.Value // controls; the only state shared with Bubble Tea
	// Everything below belongs exclusively to the Ebitengine callback.
	sheet   *ebiten.Image
	frames  [4]*ebiten.Image
	white   *ebiten.Image
	last    time.Time
	elapsed float64
}

func newScene(c controls) *scene {
	s := &scene{}
	s.controls.Store(c)
	return s
}

// spriteSheet builds four transparent 16x16 walking frames side by side.
// The neutral palette lets DrawImage tint the same artwork for every robot.
// Replace this with a decoded PNG to use an artist-created sprite sheet.
func spriteSheet() *image.RGBA {
	sheet := image.NewRGBA(image.Rect(0, 0, 4*spriteSize, spriteSize))
	for frame := 0; frame < 4; frame++ {
		x := frame * spriteSize
		bob := frame % 2
		box := func(x0, y0, x1, y1 int, c color.RGBA) {
			r := image.Rect(x+x0, y0, x+x1, y1)
			draw.Draw(sheet, r, image.NewUniform(c), image.Point{}, draw.Src)
		}
		light := color.RGBA{R: 255, G: 255, B: 255, A: 255}
		shade := color.RGBA{R: 150, G: 150, B: 150, A: 255}
		dark := color.RGBA{R: 24, G: 30, B: 46, A: 255}
		box(7, bob, 9, 3+bob, shade) // antenna
		box(7, bob, 9, 1+bob, light)
		box(3, 3+bob, 13, 9+bob, light) // head
		box(4, 5+bob, 12, 8+bob, dark)  // visor
		box(8, 5+bob, 9, 7+bob, light)  // eyes looking right
		box(10, 5+bob, 11, 7+bob, light)
		box(5, 9+bob, 11, 13+bob, shade) // body
		box(6, 10+bob, 10, 12+bob, light)
		box(2, 9+bob, 4, 12+bob, light) // arms
		box(12, 8+bob, 14, 11+bob, light)
		step := [...]int{-1, 0, 1, 0}[frame]
		box(5+step, 13, 7+step, 16, light)
		box(9-step, 13, 11-step, 16, light)
	}
	return sheet
}

func (s *scene) draw(dst *ebiten.Image, geo ntpicture.RasterGeometry) error {
	if s.sheet == nil {
		// GPU assets are created once, inside the running Ebitengine loop.
		s.sheet = ebiten.NewImageFromImage(spriteSheet())
		for i := range s.frames {
			s.frames[i] = s.sheet.SubImage(image.Rect(i*spriteSize, 0, (i+1)*spriteSize, spriteSize)).(*ebiten.Image)
		}
		s.white = ebiten.NewImage(1, 1)
		s.white.Fill(color.White)
	}
	c := s.controls.Load().(controls)
	now := time.Now()
	if !s.last.IsZero() && !c.paused {
		s.elapsed += now.Sub(s.last).Seconds()
	}
	s.last = now
	s.render(dst, geo, c.count, s.elapsed)
	return nil
}

func (s *scene) render(dst *ebiten.Image, geo ntpicture.RasterGeometry, count int, elapsed float64) {
	dst.Fill(color.RGBA{R: 15, G: 18, B: 35, A: 255})
	// World units are physically square, including in half-block glyph mode.
	const worldHeight = 80.0
	worldWidth := worldHeight * geo.DisplayAspect
	sx, sy := float64(geo.Pixels.X)/worldWidth, float64(geo.Pixels.Y)/worldHeight
	rect := func(x, y, w, h float64, c color.RGBA) {
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(w*sx, h*sy)
		op.GeoM.Translate(x*sx, y*sy)
		op.ColorScale.ScaleWithColor(c)
		dst.DrawImage(s.white, op)
	}
	// Deterministic stars and three platforms keep the scene stable on resize.
	for i := 0; i < 45; i++ {
		x := float64((i*73+11)%997) / 997 * worldWidth
		y := float64((i*37 + 7) % 79)
		v := uint8(60 + 25*(1+math.Sin(elapsed+float64(i))))
		rect(x, y, 0.6, 0.6, color.RGBA{R: v, G: v, B: v + 35, A: 255})
	}
	palette := [...]color.RGBA{
		{R: 94, G: 234, B: 212, A: 255},
		{R: 251, G: 146, B: 186, A: 255},
		{R: 253, G: 218, B: 110, A: 255},
		{R: 153, G: 179, B: 255, A: 255},
	}
	for lane := 0; lane < 3; lane++ {
		floor := float64(24 + lane*26)
		rect(0, floor, worldWidth, 1, color.RGBA{R: 61, G: 70, B: 99, A: 255})
		rect(0, floor+1, worldWidth, 3, color.RGBA{R: 24, G: 29, B: 50, A: 255})
		n := (count + 2 - lane) / 3
		for j := 0; j < n; j++ {
			i := j*3 + lane
			left := lane == 1
			span := worldWidth + spriteSize
			x := math.Mod(float64(j)*span/float64(n)+elapsed*float64(10+lane*3), span) - spriteSize
			if left {
				x = worldWidth - spriteSize - x
			}
			frame := (int(elapsed*8) + i) % len(s.frames)
			op := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
			if left {
				op.GeoM.Scale(-1, 1)
				op.GeoM.Translate(spriteSize, 0)
			}
			op.GeoM.Translate(math.Floor(x), floor-spriteSize)
			op.GeoM.Scale(sx, sy)
			op.ColorScale.ScaleWithColor(palette[i%len(palette)])
			dst.DrawImage(s.frames[frame], op)
		}
	}
}
