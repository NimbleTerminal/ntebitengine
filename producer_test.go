package ntebitengine

import (
	"context"
	"errors"
	"image"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/hajimehoshi/ebiten/v2"
	"nimbleterminal.dev/ntpicture"
)

func testProducer(t *testing.T) *Producer {
	t.Helper()
	pic := ntpicture.New(ntpicture.Options{StableID: true, RetireDelay: -1, Render: ntpicture.Config{Fit: ntpicture.FitFill}})
	pic.SetSize(12, 6)
	p := New(pic, func(*ebiten.Image, ntpicture.RasterGeometry) error { return nil }, Options{ManualRun: true})
	t.Cleanup(func() { p.Close()(); pic.Close() })
	return p
}

// Init synchronously reserves the first request. These tests inject its reply
// on the event loop; they exercise pacing against a real glyph Picture without
// launching a graphics loop or relying on timing.
func readyFrame(p *Producer) frameMsg {
	r := p.active
	return frameMsg{owner: p, request: r, img: image.NewRGBA(image.Rectangle{Max: r.geometry.Pixels})}
}

func TestPresentationPacing(t *testing.T) {
	p := testProducer(t)
	p.Init()
	r := p.active
	if r == nil || !p.Status().InFlight {
		t.Fatal("Init did not reserve a frame")
	}
	if p.Init() != nil || p.active != r {
		t.Fatal("Init is not idempotent")
	}
	p.Update("unrelated")
	if p.active != r {
		t.Fatal("unrelated event requested another frame")
	}

	// Drive real glyph preparation and commitment, then deliver PresentedMsg.
	cmd := p.Update(readyFrame(p))
	if p.Status().InFlight || !p.Status().AwaitingPresentation || p.Status().Frames != 1 {
		t.Fatalf("bad submission state: %+v", p.Status())
	}
	p.Update(ntpicture.PresentedMsg{Owner: ntpicture.New(ntpicture.Options{}), Generation: 999})
	p.Update(ntpicture.PresentedMsg{Owner: p.pic, Generation: p.submitted - 1})
	if p.active != nil {
		t.Fatal("foreign or old presentation opened render slot")
	}
	prepared := cmd()
	committed := p.pic.Update(prepared)
	if committed == nil {
		t.Fatal("picture did not commit glyph frame")
	}
	p.Update(prepared)
	presented, ok := committed().(ntpicture.PresentedMsg)
	if !ok {
		t.Fatal("expected PresentedMsg")
	}
	p.Update(presented)
	if p.active == nil || p.active == r || p.Status().AwaitingPresentation {
		t.Fatal("presentation did not request the next frame")
	}
	current := p.active
	p.Update(presented)
	if p.active != current {
		t.Fatal("duplicate presentation requested concurrent work")
	}
}

func TestResizeDrainsCanceledRender(t *testing.T) {
	p := testProducer(t)
	p.Init()
	old := readyFrame(p)
	for size := 13; size < 20; size++ {
		p.pic.SetSize(size, 7)
		p.Update(tea.WindowSizeMsg{Width: size, Height: 7})
		if p.active != old.request || !p.Status().InFlight {
			t.Fatal("resize opened another render slot before old render returned")
		}
	}
	if old.request.ctx.Err() == nil {
		t.Fatal("resize did not cancel request")
	}
	p.Update(old)
	if p.Status().Frames != 0 || p.active == nil || p.active == old.request || p.active.geometry != p.pic.RasterGeometry() {
		t.Fatalf("obsolete frame accepted or replacement missing: %+v", p.Status())
	}
	current := p.active
	p.Update(old)
	if p.active != current {
		t.Fatal("late duplicate drained current render")
	}
}

func TestViewportDoesNotInvalidate(t *testing.T) {
	p := testProducer(t)
	p.Init()
	before := p.Status()
	p.pic.SetViewport(ntpicture.Viewport{X: 2, Y: 1, Width: 4, Height: 3})
	p.Update("viewport moved")
	if p.Status() != before || p.active.ctx.Err() != nil {
		t.Fatal("viewport invalidated raster")
	}
	p.Update(readyFrame(p))
	before = p.Status()
	p.pic.SetViewport(ntpicture.Viewport{X: 3, Width: 4, Height: 3})
	p.Update("viewport moved while awaiting")
	if p.Status() != before || p.active != nil {
		t.Fatal("viewport bypassed presentation pacing")
	}
}

func TestEmptyGeometryAndModeChanges(t *testing.T) {
	p := testProducer(t)
	p.pic.SetSize(0, 0)
	p.Init()
	if p.active != nil {
		t.Fatal("rendered empty raster")
	}
	p.pic.SetSize(10, 5)
	p.Update(nil)
	old := readyFrame(p)
	p.pic.SetTerminal(true, 8, 16)
	p.Update(nil)
	if old.request.ctx.Err() == nil {
		t.Fatal("mode change did not invalidate")
	}
	p.Update(old)
	old = readyFrame(p)
	p.pic.SetResolution(0.5)
	p.Update(nil)
	if old.request.ctx.Err() == nil {
		t.Fatal("resolution change did not invalidate")
	}
	p.pic.SetSize(0, 0)
	p.Update(old)
	if p.active != nil || p.Status().InFlight {
		t.Fatal("empty raster did not stop production")
	}
}

func TestErrorsPauseUntilGeometryChanges(t *testing.T) {
	for _, source := range []string{"draw", "picture"} {
		t.Run(source, func(t *testing.T) {
			p := testProducer(t)
			p.Init()
			want := errors.New("test error")
			var msg tea.Msg
			if source == "draw" {
				frame := readyFrame(p)
				frame.err = want
				msg = frame
			} else {
				p.Update(readyFrame(p))
				msg = ntpicture.ErrorMsg{Owner: p.pic, Generation: p.submitted, Err: want}
			}
			cmd := p.Update(msg)
			reported, ok := cmd().(ErrorMsg)
			if !ok || reported.Owner != p || !errors.Is(reported.Err, want) {
				t.Fatal("error not surfaced")
			}
			p.Update(reported)
			p.Update(ntpicture.PresentedMsg{Owner: p.pic, Generation: p.submitted})
			if p.active != nil || p.Status().Err != want {
				t.Fatal("error did not pause producer")
			}
			p.pic.SetSize(13, 6)
			p.Update(nil)
			if p.active == nil || p.Status().Err != nil {
				t.Fatal("geometry change did not recover")
			}
		})
	}
}

func TestStaleFrameRestartsAndForeignMessagesAreIgnored(t *testing.T) {
	p := testProducer(t)
	p.Init()
	p.Update(readyFrame(p))
	foreign := testProducer(t)
	foreign.Init()
	p.Update(readyFrame(foreign))
	p.Update(ntpicture.StaleFrameMsg{Owner: foreign.pic})
	p.Update(ntpicture.ErrorMsg{Owner: foreign.pic, Generation: 999, Err: errors.New("foreign")})
	if !p.Status().AwaitingPresentation || p.Status().Err != nil {
		t.Fatal("foreign message changed state")
	}
	p.Update(ntpicture.StaleFrameMsg{Owner: p.pic, Geometry: p.pic.RasterGeometry()})
	if p.active == nil || p.Status().AwaitingPresentation {
		t.Fatal("stale frame stalled producer")
	}
}

func TestCloseDropsLateFrame(t *testing.T) {
	p := testProducer(t)
	p.Init()
	frame := readyFrame(p)
	p.Close()()
	p.Close()()
	if frame.request.ctx.Err() == nil {
		t.Fatal("Close did not cancel")
	}
	if p.Update(frame) != nil || p.Init() != nil || p.Status().Frames != 0 || !p.Status().Closed {
		t.Fatal("closed producer accepted work")
	}
	if err := p.Run(); err != nil {
		t.Fatalf("Run after early Close: %v", err)
	}
}

func TestInvalidInputs(t *testing.T) {
	for _, p := range []*Producer{New(nil, nil, Options{}), New(ntpicture.New(ntpicture.Options{}), nil, Options{})} {
		if _, ok := p.Init()().(ErrorMsg); !ok {
			t.Fatal("invalid input did not report error")
		}
		p.Close()()
	}
}

func TestCommandCancellationWaitsForWorker(t *testing.T) {
	p := testProducer(t)
	p.initialized = true
	p.status.Geometry = p.pic.RasterGeometry()
	cmd := p.start()
	returned := make(chan tea.Msg, 1)
	go func() { returned <- cmd() }()
	r := receive(t, p.loop.requests)
	p.pic.SetSize(30, 10)
	p.Update(nil)
	select {
	case <-returned:
		t.Fatal("command returned before canceled worker drained")
	default:
	}
	r.reply <- renderResult{err: r.ctx.Err()}
	p.Update(receive(t, returned))
	if p.active == r || p.active == nil {
		t.Fatal("worker completion did not free slot")
	}
}

func TestLoopLifecycle(t *testing.T) {
	p := testProducer(t)
	entered := make(chan struct{})
	exit := make(chan struct{})
	want := errors.New("driver failed")
	p.loop.runGame = func(ebiten.Game) error { close(entered); <-exit; return want }
	finished := make(chan error, 1)
	go func() { finished <- p.Run() }()
	receive(t, entered)
	if err := p.Run(); err == nil {
		t.Fatal("duplicate Run allowed")
	}
	close(exit)
	if err := receive(t, finished); err != want {
		t.Fatalf("Run error = %v", err)
	}
	p.Init()
	p.Update(loopStoppedMsg{owner: p, err: want})
	p.pic.SetSize(30, 10)
	p.Update(nil)
	if !p.stopped || p.Status().InFlight || p.Status().Err != want {
		t.Fatal("loop failure was not terminal")
	}
}

func TestLoopFailureWhileRequestWaiting(t *testing.T) {
	p := testProducer(t)
	p.manual = false
	entered, exit := make(chan struct{}), make(chan struct{})
	want := errors.New("driver initialization failed")
	p.loop.runGame = func(ebiten.Game) error { close(entered); <-exit; return want }
	cmds := p.Init()().(tea.BatchMsg)
	returned := make(chan tea.Msg, len(cmds))
	for _, cmd := range cmds {
		go func() { returned <- cmd() }()
	}
	receive(t, entered)
	close(exit)
	reports := 0
	for range cmds {
		msg := receive(t, returned)
		if cmd := p.Update(msg); cmd != nil {
			reported, ok := cmd().(ErrorMsg)
			if !ok || reported.Err != want {
				t.Fatalf("loop failure was masked: %#v", reported)
			}
			reports++
		}
	}
	if reports != 1 || !p.stopped || p.Status().Err != want {
		t.Fatal("loop failure did not stop producer exactly once")
	}
}

func TestCloseWaitsForRunningLoop(t *testing.T) {
	p := testProducer(t)
	entered, release := make(chan struct{}), make(chan struct{})
	p.loop.runGame = func(ebiten.Game) error {
		close(entered)
		<-p.loop.stop
		<-release // models a callback that has not returned yet
		return nil
	}
	finished := make(chan error, 1)
	go func() { finished <- p.Run() }()
	receive(t, entered)
	closeCmd := p.Close()
	select {
	case <-p.loop.done:
		t.Fatal("Close completed while loop was running")
	default:
	}
	closed := make(chan tea.Msg, 1)
	go func() { closed <- closeCmd() }()
	close(release)
	receive(t, closed)
	if err := receive(t, finished); err != nil {
		t.Fatal(err)
	}
}

func TestCanceledRequestSkipsDraw(t *testing.T) {
	l := newGameLoop(func(*ebiten.Image, ntpicture.RasterGeometry) error {
		t.Fatal("canceled request invoked draw")
		return nil
	}, Options{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := &request{ctx: ctx, reply: make(chan renderResult, 1)}
	l.requests <- r
	g := &frameGame{loop: l}
	if err := g.Update(); err != nil {
		t.Fatal(err)
	}
	if got := receive(t, r.reply); got.err != context.Canceled || got.img != nil {
		t.Fatal("canceled request was rendered")
	}
	l.close()
	if err := g.Update(); !errors.Is(err, ebiten.Termination) {
		t.Fatalf("shutdown = %v", err)
	}
}

func receive[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case value := <-ch:
		return value
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for worker")
		var zero T
		return zero
	}
}
