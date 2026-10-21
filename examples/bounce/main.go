// A terminal framebuffer drawn by a hidden Ebitengine loop.
package main

import (
	"errors"
	"fmt"
	"image/color"
	"math"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"nimbleterminal.dev/ntebitengine"
	"nimbleterminal.dev/ntebitengine/examples/internal/demorun"
	"nimbleterminal.dev/ntpicture"
)

type model struct {
	pic      *ntpicture.Picture
	producer *ntebitengine.Producer
	err      error
}

func newModel() *model {
	pic := ntpicture.New(ntpicture.Options{
		AutoMode: true,
		StableID: true,
		Render:   ntpicture.Config{Fit: ntpicture.FitFill, KittyResolutionFactor: 0.5},
	})
	started := time.Now()
	draw := func(dst *ebiten.Image, geo ntpicture.RasterGeometry) error {
		dst.Fill(color.RGBA{R: 16, G: 22, B: 38, A: 255})
		t := time.Since(started).Seconds()
		w, h := float64(geo.Pixels.X), float64(geo.Pixels.Y)
		// Draw in physical display coordinates. Glyph pixels are generally
		// not square; DisplayAspect keeps the ball round in either mode.
		radius := 0.12 * math.Min(1, geo.DisplayAspect)
		rx, ry := radius/geo.DisplayAspect*w, radius*h
		x := w/2 + (w/2-rx)*math.Sin(t*1.7)
		y := h/2 + (h/2-ry)*math.Sin(t*2.3)
		var path vector.Path
		path.MoveTo(float32(x+rx), float32(y))
		for i := 1; i <= 64; i++ {
			a := float64(i) * 2 * math.Pi / 64
			path.LineTo(float32(x+rx*math.Cos(a)), float32(y+ry*math.Sin(a)))
		}
		path.Close()
		opts := &vector.DrawPathOptions{AntiAlias: true}
		opts.ColorScale.ScaleWithColor(color.RGBA{R: 80, G: 220, B: 240, A: 255})
		vector.FillPath(dst, &path, nil, opts)
		return nil
	}
	return &model{pic: pic, producer: ntebitengine.New(pic, draw, ntebitengine.Options{ManualRun: true})}
}

func (m *model) Init() tea.Cmd {
	return tea.Batch(m.pic.Init(), m.producer.Init())
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		cmds = append(cmds, m.pic.SetSize(max(0, msg.Width), max(0, msg.Height-1)))
	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Sequence(m.producer.Close(), m.pic.Close(), tea.Quit)
		case "m":
			cmds = append(cmds, m.pic.ToggleMode())
		case "+", "=":
			cmds = append(cmds, m.pic.SetResolution(min(1, m.pic.Resolution()+0.25)))
		case "-":
			cmds = append(cmds, m.pic.SetResolution(max(0.25, m.pic.Resolution()-0.25)))
		}
	case ntebitengine.ErrorMsg:
		if msg.Owner == m.producer {
			m.err = msg.Err
			return m, tea.Sequence(m.producer.Close(), m.pic.Close(), tea.Quit)
		}
	}
	// Picture first: terminal discovery can change the raster geometry.
	cmds = append(cmds, m.pic.Update(msg))
	cmds = append(cmds, m.producer.Update(msg))
	return m, tea.Batch(cmds...)
}

func (m *model) View() tea.View {
	g := m.pic.RasterGeometry()
	v := tea.NewView(fmt.Sprintf("%dx%d px | m: mode | +/-: resolution | q: quit\n%s", g.Pixels.X, g.Pixels.Y, m.pic.View()))
	v.AltScreen = true
	return v
}

func run() error {
	m := newModel()
	done := make(chan error, 1)
	go func() {
		err := demorun.Run(m)
		// Also stop the GPU if Tea fails before Init or exits via a signal.
		m.producer.Close()()
		_ = m.pic.Close()
		done <- errors.Join(err, m.err)
	}()
	// Ebitengine binds the main goroutine to the main OS thread.
	loopErr := m.producer.Run()
	return errors.Join(loopErr, <-done)
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
