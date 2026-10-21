package main

import (
	"bytes"
	"fmt"
	"image/color"
	"math"
	"strings"
	"sync/atomic"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/goregular"
	"nimbleterminal.dev/ntpicture"
)

var fontNames = [...]string{"Go Sans", "Go Bold", "Go Mono"}
var layoutNames = [...]string{"specimen", "paragraph", "motion"}
var samples = [...]string{"Hello, terminal.", "Make type move.", "Hello, κόσμε!", "Привет, мир!"}

type controls struct {
	font, layout, align, sample int
	scale                       float64
	paused                      bool
}

type scene struct {
	controls atomic.Value              // immutable controls shared with Bubble Tea
	sources  [3]*text.GoTextFaceSource // font caches belong to the draw callback
	last     time.Time
	elapsed  float64
}

func newScene(c controls) *scene {
	s := &scene{}
	s.controls.Store(c)
	return s
}

func (s *scene) draw(dst *ebiten.Image, geo ntpicture.RasterGeometry) error {
	if s.sources[0] == nil {
		var sources [3]*text.GoTextFaceSource
		for i, data := range [][]byte{goregular.TTF, gobold.TTF, gomono.TTF} {
			var err error
			sources[i], err = text.NewGoTextFaceSource(bytes.NewReader(data))
			if err != nil {
				return fmt.Errorf("load %s: %w", fontNames[i], err)
			}
		}
		s.sources = sources
	}
	c := s.controls.Load().(controls)
	now := time.Now()
	if !s.last.IsZero() && !c.paused {
		s.elapsed += now.Sub(s.last).Seconds()
	}
	s.last = now
	s.render(dst, geo, c)
	return nil
}

var (
	ink    = color.RGBA{R: 240, G: 235, B: 223, A: 255}
	muted  = color.RGBA{R: 139, G: 155, B: 172, A: 255}
	accent = color.RGBA{R: 251, G: 191, B: 112, A: 255}
)

// Typesetting uses a physical coordinate system. Faces are rasterized at the
// target's Y scale; only X is corrected for non-square terminal raster pixels.
// This avoids stretching glyphs and avoids rendering large fonts only to shrink
// them to a tiny glyph framebuffer afterwards.
type canvas struct {
	dst       *ebiten.Image
	w, sx, sy float64
	source    *text.GoTextFaceSource
}

func (c canvas) face(size float64) *text.GoTextFace {
	return &text.GoTextFace{Source: c.source, Size: size * c.sy}
}

func (c canvas) label(value string, size, x, y float64, clr color.Color, align text.Align) {
	op := &text.DrawOptions{}
	op.PrimaryAlign = align
	op.LineSpacing = size * c.sy * 1.45
	op.GeoM.Scale(c.sx/c.sy, 1)
	op.GeoM.Translate(x*c.sx, y*c.sy)
	op.ColorScale.ScaleWithColor(clr)
	text.Draw(c.dst, value, c.face(size), op)
}

func (c canvas) fit(value string, size, width float64) float64 {
	w, _ := text.Measure(value, c.face(size), 0)
	if w > width*c.sy {
		return size * width * c.sy / w
	}
	return size
}

func (c canvas) rect(x, y, w, h float64, clr color.Color) {
	vector.FillRect(c.dst, float32(x*c.sx), float32(y*c.sy), float32(w*c.sx), float32(h*c.sy), clr, false)
}

func (s *scene) render(dst *ebiten.Image, geo ntpicture.RasterGeometry, controls controls) {
	dst.Fill(color.RGBA{R: 17, G: 25, B: 37, A: 255})
	const height = 360.0
	c := canvas{dst: dst, w: height * geo.DisplayAspect, sy: float64(geo.Pixels.Y) / height, source: s.sources[controls.font]}
	c.sx = float64(geo.Pixels.X) / c.w
	margin := math.Min(28, c.w*0.07)
	width := c.w - 2*margin
	caption := samples[controls.sample]
	align := text.Align(controls.align)
	x := margin
	if align == text.AlignCenter {
		x = c.w / 2
	}
	if align == text.AlignEnd {
		x = c.w - margin
	}

	label := fmt.Sprintf("0%d / %s", controls.layout+1, strings.ToUpper(layoutNames[controls.layout]))
	c.label(label, c.fit(label, 12, width), margin, 18, accent, text.AlignStart)
	c.rect(margin, 44, width, 1, color.RGBA{R: 49, G: 62, B: 79, A: 255})
	switch controls.layout {
	case 0:
		size := c.fit(caption, math.Min(70, 64*controls.scale), width)
		// A subtle shadow is a second draw of the same cached glyphs.
		c.label(caption, size, x+2, 77, color.RGBA{R: 4, G: 9, B: 18, A: 255}, align)
		c.label(caption, size, x, 74, ink, align)
		alphabet := "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
		c.label(alphabet, c.fit(alphabet, 23*controls.scale, width), x, 161, muted, align)
		digits := "0123456789  !? & @ # %"
		c.label(digits, c.fit(digits, 22*controls.scale, width), x, 195, accent, align)
		// Three faces share one layout; the selected face is highlighted.
		cardWidth := (width - 16) / 3
		if cardWidth <= 8 {
			return // An extremely narrow terminal has no room for font cards.
		}
		for i, source := range s.sources {
			left := margin + float64(i)*(cardWidth+8)
			bg := color.RGBA{R: 25, G: 36, B: 51, A: 255}
			if i == controls.font {
				bg = color.RGBA{R: 46, G: 51, B: 56, A: 255}
			}
			c.rect(left, 247, cardWidth, 86, bg)
			card := c
			card.source = source
			card.label("Aa", card.fit("Aa", 40, cardWidth-8), left+cardWidth/2, 251, ink, text.AlignCenter)
			card.label(fontNames[i], card.fit(fontNames[i], 11, cardWidth-8), left+cardWidth/2, 310, muted, text.AlignCenter)
		}
	case 1:
		c.label(caption, c.fit(caption, math.Min(48, 38*controls.scale), width), x, 65, ink, align)
		const paragraph = "Good type gives a scene its voice. This paragraph is measured with the active font and wrapped to the framebuffer's physical width. Change the font, alignment, or size; resize your terminal and watch the lines find a new rhythm."
		size := 19 * controls.scale
		lines := wrap(paragraph, c.face(size), width*c.sy)
		maxLines := max(1, int(175/(size*1.45)))
		if len(lines) > maxLines {
			lines = lines[:maxLines]
			// Replace the last line with a width-bounded truncation marker.
			lines[len(lines)-1] = "…"
		}
		c.label(strings.Join(lines, "\n"), size, x, 131, muted, align)
		info := "MEASURE → WRAP → ALIGN"
		c.label(info, c.fit(info, 12, width), margin, 325, accent, text.AlignStart)
	case 2:
		size := c.fit(caption, 52*controls.scale, width/1.2)
		for layer := 4; layer >= 0; layer-- {
			phase := s.elapsed - float64(layer)*0.12
			op := &text.DrawOptions{}
			op.PrimaryAlign, op.SecondaryAlign = text.AlignCenter, text.AlignCenter
			scale := 0.92 + 0.08*math.Sin(phase*1.4)
			op.GeoM.Scale(scale, scale)
			op.GeoM.Rotate(0.12 * math.Sin(phase))
			op.GeoM.Scale(c.sx/c.sy, 1)
			op.GeoM.Translate(c.w/2*c.sx, (180+22*math.Sin(phase*1.8))*c.sy)
			op.ColorScale.ScaleWithColor(color.NRGBA{R: 251, G: 191, B: 112, A: uint8(255 / (1 + layer*3))})
			op.Filter = ebiten.FilterLinear
			text.Draw(dst, caption, c.face(size), op)
		}
		c.label("Aa", 50, c.w/2, 252, ink, text.AlignCenter)
		note := "CACHED GLYPHS / LIVE TRANSFORMS"
		c.label(note, c.fit(note, 12, width), c.w/2, 325, muted, text.AlignCenter)
	}
}

// wrap measures candidate lines rather than estimating width by rune count.
// This demo wraps on spaces; full Unicode line-breaking belongs in a layout
// library. An overlong word is split at rune boundaries to keep it in bounds.
func wrap(value string, face text.Face, width float64) []string {
	var lines []string
	line := ""
	for _, word := range strings.Fields(value) {
		candidate := word
		if line != "" {
			candidate = line + " " + word
		}
		if w, _ := text.Measure(candidate, face, 0); w <= width {
			line = candidate
			continue
		}
		if line != "" {
			lines = append(lines, line)
			line = ""
		}
		for _, r := range word {
			candidate = line + string(r)
			if w, _ := text.Measure(candidate, face, 0); w > width && line != "" {
				lines = append(lines, line)
				line = string(r)
			} else {
				line = candidate
			}
		}
	}
	if line != "" {
		lines = append(lines, line)
	}
	return lines
}
