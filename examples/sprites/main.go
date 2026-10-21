// Animated sprites presented in a terminal through ntpicture.
package main

import (
	"errors"
	"fmt"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"
	"nimbleterminal.dev/ntebitengine"
	"nimbleterminal.dev/ntpicture"
)

type model struct {
	pic      *ntpicture.Picture
	producer *ntebitengine.Producer
	scene    *scene
	controls controls
	err      error

	fps           float64
	fpsSince      time.Time
	fpsFrames     int
	lastPresented uint64
}

type fpsTickMsg struct{}

func fpsTick() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(time.Time) tea.Msg { return fpsTickMsg{} })
}

func newModel() *model {
	pic := ntpicture.New(ntpicture.Options{
		AutoMode: true,
		StableID: true,
		Render:   ntpicture.Config{Fit: ntpicture.FitFill, KittyResolutionFactor: 0.5},
	})
	c := controls{count: 12}
	s := newScene(c)
	return &model{pic: pic, scene: s, controls: c,
		producer: ntebitengine.New(pic, s.draw, ntebitengine.Options{ManualRun: true})}
}

func (m *model) Init() tea.Cmd {
	m.fpsSince = time.Now()
	return tea.Batch(m.pic.Init(), m.producer.Init(), fpsTick())
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case ntpicture.PresentedMsg:
		if msg.Owner == m.pic && msg.Generation > m.lastPresented {
			m.lastPresented = msg.Generation
			m.fpsFrames++
		}
	case fpsTickMsg:
		now := time.Now()
		if elapsed := now.Sub(m.fpsSince).Seconds(); elapsed > 0 {
			m.fps = float64(m.fpsFrames) / elapsed
		}
		m.fpsFrames, m.fpsSince = 0, now
		if !m.producer.Status().Closed {
			cmds = append(cmds, fpsTick())
		}
	case tea.WindowSizeMsg:
		cmds = append(cmds, m.pic.SetSize(max(0, msg.Width), max(0, msg.Height-2)))
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
		case "space":
			m.controls.paused = !m.controls.paused
		case "]":
			m.controls.count = min(60, m.controls.count+3)
		case "[":
			m.controls.count = max(3, m.controls.count-3)
		}
		// Publish a value snapshot; the draw callback owns all animation/GPU state.
		m.scene.controls.Store(m.controls)
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
	state := "walking"
	if m.controls.paused {
		state = "paused"
	}
	v := tea.NewView(fmt.Sprintf("SPRITE PARADE | %d robots | %s | %dx%d px | %.1f FPS\nspace: pause | [/]: count | m: mode | +/-: resolution | q: quit\n%s",
		m.controls.count, state, g.Pixels.X, g.Pixels.Y, m.fps, m.pic.View()))
	v.AltScreen = true
	return v
}

func run() error {
	m := newModel()
	done := make(chan error, 1)
	go func() {
		_, err := tea.NewProgram(m).Run()
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
