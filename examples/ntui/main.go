// Floating ntui controls composed with the shader lab before the final readback.
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
	"nimbleterminal.dev/ntebitengine/examples/internal/shaderlab"
	"nimbleterminal.dev/ntpicture"
	"nimbleterminal.dev/ntui"
)

const headerRows = 2

type model struct {
	pic           *ntpicture.Picture
	producer      *ntebitengine.Producer
	scene         *scene
	ui            *ntui.Context
	glyphBackend  *ntui.GlyphBackend
	controls      shaderlab.Controls
	width, height int
	glyph         bool
	glyphRows     int
	note          string
	light         bool
	phase         float64
	lastTick      time.Time
	samples       []float64
	fps           float64
	fpsSince      time.Time
	fpsFrames     int
	lastPresented uint64
	err           error
}
type tickMsg time.Time

func tick() tea.Cmd {
	return tea.Tick(time.Second/30, func(now time.Time) tea.Msg { return tickMsg(now) })
}
func newModel() *model {
	pic := ntpicture.New(ntpicture.Options{AutoMode: true, StableID: true, Render: ntpicture.Config{Fit: ntpicture.FitFill, KittyResolutionFactor: 1}})
	m := &model{pic: pic, scene: newScene(), ui: ntui.New(ntui.WithTheme(ntui.HUDTheme())), controls: shaderlab.Controls{Strength: 1}, width: 80, height: 26, note: "Type here: scene shortcuts stay captured"}
	m.producer = ntebitengine.New(pic, m.scene.draw, ntebitengine.Options{ManualRun: true})
	return m
}
func (m *model) Init() tea.Cmd {
	m.lastTick = time.Now()
	m.fpsSince = m.lastTick
	layout := m.layout()
	m.frame()
	m.publish()
	return tea.Batch(layout, m.pic.Init(), m.producer.Init(), tick())
}
func readableRaster(g ntpicture.RasterGeometry) bool {
	return g.Mode != ntpicture.PictureGlyph && g.Cells.X > 0 && g.Cells.Y > 0 && float64(g.Pixels.X)/float64(g.Cells.X) >= 6 && float64(g.Pixels.Y)/float64(g.Cells.Y) >= 12
}
func (m *model) layout() tea.Cmd {
	// Pixels-per-cell depends on mode, cell metrics, and resolution. The reserved
	// text pane doesn't affect that ratio. Reserve a scene row even on tiny screens.
	m.glyph = !readableRaster(m.pic.RasterGeometry())
	m.glyphRows = 0
	if m.glyph {
		m.glyphRows = min(16, max(1, m.height-headerRows-1))
	}
	cmd := m.pic.SetSize(max(1, m.width), max(1, m.height-headerRows-m.glyphRows))
	rows := max(1, m.height-headerRows)
	if m.glyph {
		rows = m.glyphRows
	}
	oldW, oldH := m.ui.ScreenSize()
	if oldW != max(1, m.width) || oldH != rows {
		// ntui remembers panel positions. Reset transient layout storage on viewport
		// changes so a previously dragged title cannot become stranded offscreen.
		m.ui.Storage = ntui.New().Storage
		m.ui.Active = 0
		m.ui.ClearFocus()
	}
	m.ui.SetScreenSize(max(1, m.width), rows)
	m.ui.Input.OffsetY = headerRows
	if m.glyph {
		if m.glyphBackend == nil {
			m.glyphBackend = ntui.NewGlyphBackend(max(1, m.width), rows)
		} else {
			m.glyphBackend.Resize(max(1, m.width), rows)
		}
	}
	return cmd
}
func (m *model) advance(now time.Time) {
	if !m.lastTick.IsZero() && !m.controls.Paused {
		m.phase += math.Max(0, math.Min(.1, now.Sub(m.lastTick).Seconds()))
	}
	m.lastTick = now
	if elapsed := now.Sub(m.fpsSince).Seconds(); elapsed >= .5 {
		m.fps = float64(m.fpsFrames) / elapsed
		m.fpsFrames = 0
		m.fpsSince = now
		m.samples = append(m.samples, math.Min(1, m.fps/60))
		if len(m.samples) > 60 {
			m.samples = m.samples[len(m.samples)-60:]
		}
	}
}
func (m *model) frame() {
	theme := ntui.HUDTheme()
	if m.light {
		theme = ntui.LightTheme()
		theme.CompactButtons = true
	}
	m.ui.Theme = theme
	u := m.ui
	w, h := u.ScreenSize()
	u.BeginFrame()
	panelW := min(40, max(12, w-2))
	panelH := min(20, max(5, h-1))
	if u.BeginPanel("Shader controls", ntui.Rect{X: 1, Y: 1, W: panelW, H: panelH}, ntui.WithPanelPadding(1, 1)) {
		u.Combo("Effect", &m.controls.Effect, shaderlab.EffectNames)
		u.Combo("Palette", &m.controls.Palette, shaderlab.PaletteNames)
		u.SliderFloat("Strength", &m.controls.Strength, .25, 2, "%.2f")
		u.Checkbox("Paused", &m.controls.Paused)
		u.Checkbox("Light panel", &m.light)
		u.TextInput("Note", &m.note, panelW-4)
		u.Separator()
		u.ProgressBar(m.controls.Strength/2, panelW-4, fmt.Sprintf("strength %.0f%%", m.controls.Strength*100))
		u.LabelMuted("Drag title; scroll for more controls")
		u.Label("Symbols: ✓ ▾ ▸ ◂ ● ○")
		u.PlotHistogram("Presented frames", m.samples, panelW-4, 3, "FPS / 60")
		u.Label("This deliberately long label is clipped by the panel edge.")
	}
	u.EndPanel()
	if w >= 74 {
		if u.BeginPanel("Telemetry", ntui.Rect{X: max(42, w-30), Y: 1, W: 28, H: min(13, max(5, h-1))}, ntui.WithCollapsible(true)) {
			u.Text("Presented FPS: %.1f", m.fps)
			g := m.pic.RasterGeometry()
			u.Text("Raster: %d × %d", g.Pixels.X, g.Pixels.Y)
			u.Text("Effect: %s", shaderlab.EffectNames[m.controls.Effect])
			u.PlotLines("History", m.samples, 24, 4, "60 FPS")
			u.LabelMuted("Drag this panel too")
		}
		u.EndPanel()
	}
	u.EndFrame()
}
func (m *model) publish() {
	m.scene.input.Store(snapshot{controls: m.controls, phase: m.phase, commands: m.ui.DrawList.Snapshot(), geometry: m.pic.RasterGeometry(), glyph: m.glyph})
}
func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	if key, ok := msg.(tea.KeyPressMsg); ok && key.String() == "ctrl+c" {
		return m, tea.Sequence(m.producer.Close(), m.pic.Close(), tea.Quit)
	}
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tickMsg:
		m.advance(time.Time(msg))
		if !m.producer.Status().Closed {
			cmds = append(cmds, tick())
		}
	case ntpicture.PresentedMsg:
		if msg.Owner == m.pic && msg.Generation > m.lastPresented {
			m.lastPresented = msg.Generation
			m.fpsFrames++
		}
	case ntebitengine.ErrorMsg:
		if msg.Owner == m.producer {
			m.err = msg.Err
			return m, tea.Sequence(m.producer.Close(), m.pic.Close(), tea.Quit)
		}
	}
	// Terminal discovery can change geometry; process it before layout/publication
	// and Producer.Update. There is still only one producer and one readback.
	cmds = append(cmds, m.pic.Update(msg))
	cmds = append(cmds, m.layout())
	captureKeys := m.ui.WantCaptureKeyboard()
	focused := m.ui.Focus != 0
	m.ui.ProcessInput(msg)
	// ProcessInput(WindowSizeMsg) uses the terminal dimensions; restore our UI viewport.
	rows := max(1, m.height-headerRows)
	if m.glyph {
		rows = m.glyphRows
	}
	m.ui.SetScreenSize(max(1, m.width), rows)
	m.frame()
	captureKeys = captureKeys || m.ui.WantCaptureKeyboard()
	if key, ok := msg.(tea.KeyPressMsg); ok && !captureKeys {
		switch key.String() {
		case "q":
			return m, tea.Sequence(m.producer.Close(), m.pic.Close(), tea.Quit)
		case "m":
			cmds = append(cmds, m.pic.ToggleMode())
		case "+", "=":
			cmds = append(cmds, m.pic.SetResolution(min(1, m.pic.Resolution()+.25)))
		case "-":
			cmds = append(cmds, m.pic.SetResolution(max(.25, m.pic.Resolution()-.25)))
		case "t":
			m.controls.Effect = (m.controls.Effect + 1) % len(shaderlab.EffectNames)
		case "p":
			m.controls.Palette = (m.controls.Palette + 1) % len(shaderlab.PaletteNames)
		case "[":
			m.controls.Strength = max(.25, m.controls.Strength-.25)
		case "]":
			m.controls.Strength = min(2, m.controls.Strength+.25)
		case "space":
			if !focused && m.ui.Focus == 0 {
				m.controls.Paused = !m.controls.Paused
			}
		}
	}
	if mouse, ok := msg.(tea.MouseClickMsg); ok && (!m.ui.WantCaptureMouse() || (m.glyph && m.ui.Input.MouseY >= m.glyphRows && m.ui.Active == 0)) {
		p := mouse.Mouse()
		sceneTop := headerRows + m.glyphRows
		if p.Button == tea.MouseLeft && p.Y >= sceneTop && p.Y < m.height && p.X >= 0 && p.X < m.width {
			m.controls.Palette = (m.controls.Palette + 1) % len(shaderlab.PaletteNames)
		}
	}
	// Rebuild without input edges after shortcuts/mode changes, then publish state
	// and commands together. No widget-bound state is touched by DrawFunc.
	cmds = append(cmds, m.layout())
	m.frame()
	m.publish()
	cmds = append(cmds, m.producer.Update(msg))
	return m, tea.Batch(cmds...)
}
func (m *model) View() tea.View {
	mode := "GPU overlay"
	if m.glyph {
		mode = "readable glyph controls (raster text disabled)"
	}
	header := fmt.Sprintf("NTUI KAGE LAB | %.1f presented FPS | %s", m.fps, mode)
	help := "drag / scroll panels · t/p: effect/palette · Space: pause · m: mode · +/-: resolution · q: quit"
	body := m.pic.View()
	if m.glyph && m.glyphBackend != nil {
		body = m.glyphBackend.RenderToString(m.ui.DrawList) + "\n" + body
	}
	v := tea.NewView(ansi.Truncate(header, m.width, "") + "\n" + ansi.Truncate(help, m.width, "") + "\n" + body)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeAllMotion
	return v
}
func run() error {
	m := newModel()
	done := make(chan error, 1)
	go func() {
		err := demorun.Run(m)
		m.producer.Close()()
		_ = m.pic.Close()
		done <- errors.Join(err, m.err)
	}()
	loopErr := m.producer.Run()
	return errors.Join(loopErr, <-done)
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
