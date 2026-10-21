package main

import (
	"image"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"nimbleterminal.dev/ntpicture"
	"nimbleterminal.dev/ntui"
)

func setup(t *testing.T) *model {
	t.Helper()
	m := newModel()
	m.layout()
	m.frame()
	m.publish()
	t.Cleanup(func() { m.producer.Close()(); _ = m.pic.Close() })
	return m
}
func key(s string) tea.KeyPressMsg { return tea.KeyPressMsg{Code: rune(s[0]), Text: s} }
func TestInputCaptureAndHeaderOffset(t *testing.T) {
	m := setup(t)
	// Click the title bar using terminal coordinates (two header rows above UI).
	before := m.controls.Palette
	m.Update(tea.MouseClickMsg{X: 10, Y: headerRows + 1, Button: tea.MouseLeft})
	if m.controls.Palette != before || m.ui.Active == 0 {
		t.Fatal("panel click leaked to scene or missed title")
	}
	m.Update(tea.MouseMotionMsg{X: 15, Y: headerRows + 3, Button: tea.MouseLeft})
	m.Update(tea.MouseReleaseMsg{X: 15, Y: headerRows + 3, Button: tea.MouseLeft})
	var moved bool
	for _, c := range m.ui.DrawList.Commands() {
		if c.Kind == ntui.CmdBox && c.Text == "▾ Shader controls" {
			moved = c.Bounds.X == 6 && c.Bounds.Y == 3
		}
	}
	if !moved {
		t.Fatal("drag did not respect header-relative cell coordinates")
	}
	// Reset positions and focus the text input through its declared ID, avoiding
	// dependence on font dimensions or a particular viewport size.
	m.ui.Storage = ntui.New().Storage
	m.frame()
	m.ui.PushID("Shader controls")
	_, id := m.ui.GetID("Note")
	m.ui.PopID()
	m.ui.SetFocus(id)
	m.frame()
	if !m.ui.WantCaptureKeyboard() {
		t.Fatal("text input did not acquire capture")
	}
	oldNote := m.note
	oldEffect := m.controls.Effect
	m.Update(key("t"))
	if m.controls.Effect != oldEffect || m.note == oldNote {
		t.Fatal("typing leaked to effect shortcut")
	}
	m.ui.ClearFocus()
	m.frame()
	m.Update(key("t"))
	if m.controls.Effect == oldEffect {
		t.Fatal("uncaptured scene shortcut ignored")
	}
}
func TestPauseAndSnapshotOwnership(t *testing.T) {
	m := setup(t)
	start := time.Unix(100, 0)
	m.lastTick = start
	m.fpsSince = start
	m.advance(start.Add(50 * time.Millisecond))
	phase := m.phase
	m.controls.Paused = true
	m.advance(start.Add(time.Second))
	if m.phase != phase {
		t.Fatal("pause advanced scene")
	}
	m.controls.Paused = false
	m.advance(start.Add(10 * time.Second))
	if m.phase > phase+.100001 {
		t.Fatal("stall not capped")
	}
	m.samples = []float64{.2, .7}
	m.frame()
	m.publish()
	old := m.scene.input.Load().(snapshot)
	expected := ntui.SnapshotCommands(old.commands)
	m.samples[0] = 1
	m.ui.BeginFrame()
	m.ui.EndFrame()
	m.controls.Effect = 2
	m.publish()
	if old.controls.Effect == 2 || len(old.commands) == 0 {
		t.Fatal("published snapshot changed")
	}
	found := false
	for i, c := range old.commands {
		if len(c.Values) > 0 {
			found = true
			if c.Values[0] != expected[i].Values[0] {
				t.Fatal("plot values alias source")
			}
		}
	}
	if !found {
		t.Fatal("no visible plot in test")
	}
}
func TestFallbackResolutionAndResize(t *testing.T) {
	for _, test := range []struct {
		g    ntpicture.RasterGeometry
		want bool
	}{
		{ntpicture.RasterGeometry{Cells: image.Pt(80, 24), Pixels: image.Pt(80, 48), Mode: ntpicture.PictureGlyph}, false},
		{ntpicture.RasterGeometry{Cells: image.Pt(80, 24), Pixels: image.Pt(480, 288), Mode: ntpicture.PictureKitty}, true},
		{ntpicture.RasterGeometry{Cells: image.Pt(80, 24), Pixels: image.Pt(479, 288), Mode: ntpicture.PictureKitty}, false},
		{ntpicture.RasterGeometry{}, false},
	} {
		if readableRaster(test.g) != test.want {
			t.Fatal(test)
		}
	}
	m := setup(t)
	m.Update(tea.WindowSizeMsg{Width: 78, Height: 23})
	m.publish()
	if !m.glyph || m.pic.RasterGeometry().Cells != image.Pt(78, 5) {
		t.Fatal("glyph pane did not reserve layout", m.pic.RasterGeometry())
	}
	if m.scene.input.Load().(snapshot).geometry != m.pic.RasterGeometry() {
		t.Fatal("stale geometry publication")
	}
	// A scene click below the fallback pane can change palette; a header cannot.
	before := m.controls.Palette
	m.Update(tea.MouseClickMsg{X: 1, Y: 0, Button: tea.MouseLeft})
	if m.controls.Palette != before {
		t.Fatal("header click hit scene")
	}
	m.Update(tea.MouseClickMsg{X: 1, Y: 20, Button: tea.MouseLeft})
	if m.controls.Palette == before {
		t.Fatal("scene click ignored")
	}
}
func TestQuitCancelsBeforeLoopStartup(t *testing.T) {
	m := setup(t)
	_, cmd := m.Update(key("q"))
	if cmd == nil || !m.producer.Status().Closed {
		t.Fatal("quit failed to close producer")
	}
}
