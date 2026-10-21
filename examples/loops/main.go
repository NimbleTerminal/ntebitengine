// Two event loops with explicit ownership and immutable state handoff.
package main

import (
	"errors"
	"fmt"
	"math"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"nimbleterminal.dev/ntebitengine"
	"nimbleterminal.dev/ntebitengine/examples/internal/demorun"
	"nimbleterminal.dev/ntpicture"
)

type model struct {
	pic           *ntpicture.Picture
	producer      *ntebitengine.Producer
	scene         *scene
	err           error
	state         snapshot
	paused        bool
	speed         float64
	ticks         uint64
	lastTick      time.Time
	presented     uint64
	lastPresented uint64
	fps           float64
	fpsSince      time.Time
	fpsFrames     uint64
}

// This tick advances application state. It never requests a render directly.
type tickMsg struct{}

func tick() tea.Cmd {
	return tea.Tick(time.Second/30, func(time.Time) tea.Msg { return tickMsg{} })
}

func newModel() *model {
	pic := ntpicture.New(ntpicture.Options{
		AutoMode: true,
		StableID: true,
		Render:   ntpicture.Config{Fit: ntpicture.FitFill, KittyResolutionFactor: 0.5},
	})
	s := newScene()
	return &model{pic: pic, scene: s, speed: 1,
		producer: ntebitengine.New(pic, s.draw, ntebitengine.Options{ManualRun: true})}
}

func (m *model) Init() tea.Cmd {
	m.lastTick = time.Now()
	m.fpsSince = m.lastTick
	return tea.Batch(m.pic.Init(), m.producer.Init(), tick())
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case ntpicture.PresentedMsg:
		if msg.Owner == m.pic && msg.Generation > m.lastPresented {
			m.lastPresented = msg.Generation
			m.presented++
		}
	case tickMsg:
		now := time.Now() // actual handling time, including event-loop delays
		m.advance(now)
		if elapsed := now.Sub(m.fpsSince).Seconds(); elapsed >= 0.5 {
			m.fps = float64(m.presented-m.fpsFrames) / elapsed
			m.fpsFrames, m.fpsSince = m.presented, now
		}
		if !m.producer.Status().Closed {
			cmds = append(cmds, tick())
		}
	case tea.WindowSizeMsg:
		cmds = append(cmds, m.pic.SetSize(max(0, msg.Width), max(0, msg.Height-5)))
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
			m.paused = !m.paused
			m.lastTick = time.Now() // no catch-up across pause/resume
		case "]":
			m.speed = min(2, m.speed+0.25)
		case "[":
			m.speed = max(0.25, m.speed-0.25)
		case "l":
			m.state.slow = !m.state.slow
		case "r":
			m.state.phase = 0
			m.lastTick = time.Now()
		}
		m.publish()

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

// advance owns simulation time on Tea's event loop. Long stalls are capped;
// this visual demo deliberately discards excess elapsed time instead of jumping.
func (m *model) advance(now time.Time) {
	dt := max(0, min(0.1, now.Sub(m.lastTick).Seconds()))
	m.lastTick = now
	m.ticks++
	if !m.paused {
		m.state.phase = math.Mod(m.state.phase+dt*m.speed, 2*math.Pi)
	}
	m.publish()
}

func (m *model) publish() {
	m.state.revision++
	m.scene.input.Store(m.state)
}

func (m *model) View() tea.View {
	g := m.pic.RasterGeometry()
	report := m.scene.report.Load().(drawReport)
	status := m.producer.Status()
	phase := "idle"
	if status.InFlight {
		phase = "rendering"
	}
	if status.AwaitingPresentation {
		phase = "presenting"
	}
	motion := "running"
	if m.paused {
		motion = "paused"
	}
	slow := "off"
	if m.state.slow {
		slow = "80ms"
	}
	rows := []string{
		fmt.Sprintf("LOOP LAB | %s | %.2fx | slow draw: %s | %.1f presented FPS", motion, m.speed, slow, m.fps),
		fmt.Sprintf("TEA  ticks: %d | latest snapshot: %d | phase: %.2f", m.ticks, m.state.revision, m.state.phase),
		fmt.Sprintf("DRAW calls: %d | sampled snapshot: %d | callback: %.1fms", report.calls, report.revision, float64(report.duration)/float64(time.Millisecond)),
		fmt.Sprintf("PIPE %s | generation: %d | submitted: %d | presented: %d", phase, status.Generation, status.Frames, m.presented),
		"space: pause | l: slow draw | [/]: speed | r: reset | m: mode | +/-: resolution | q: quit",
	}
	content := ""
	for _, row := range rows {
		content += ansi.Truncate(row, g.Cells.X, "…") + "\n"
	}
	v := tea.NewView(content + m.pic.View())
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
