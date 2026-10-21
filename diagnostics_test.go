package ntebitengine

import (
	"context"
	"errors"
	"image"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"nimbleterminal.dev/ntpicture"
)

func TestDiagnosticsFollowConfirmedFrame(t *testing.T) {
	p := testProducer(t)
	now := time.Unix(100, 0)
	p.now = func() time.Time { return now }
	p.initialized = true
	p.status.Geometry = p.pic.RasterGeometry()
	cmd := p.start()
	returned := make(chan tea.Msg, 1)
	go func() { returned <- cmd() }()
	r := receive(t, p.loop.requests)
	img := image.NewRGBA(image.Rectangle{Max: r.geometry.Pixels})
	r.reply <- renderResult{img: img, drawTime: 8 * time.Millisecond, readbackTime: 3 * time.Millisecond}
	// Exercise the command handoff, real Picture preparation, and commitment.
	prepare := p.Update(receive(t, returned))
	prepared := prepare()
	commit := p.pic.Update(prepared)
	p.Update(prepared)
	if commit == nil {
		t.Fatal("no Picture commit")
	}
	if p.Status().LastFrame.Generation != 0 || p.Status().PresentedFrames != 0 {
		t.Fatal("submission was counted as presentation")
	}
	now = now.Add(17 * time.Millisecond)
	p.Update(ntpicture.PresentedMsg{Owner: p.pic, Generation: p.submitted - 1})
	p.Update(ntpicture.PresentedMsg{Generation: p.submitted})
	if p.Status().LastFrame.Generation != 0 {
		t.Fatal("old or foreign presentation created a sample")
	}
	presented := commit().(ntpicture.PresentedMsg)
	p.Update(presented)
	want := FrameStats{
		Generation: r.generation, Geometry: r.geometry,
		DrawTime: 8 * time.Millisecond, ReadbackTime: 3 * time.Millisecond,
		PresentTime: 17 * time.Millisecond, Bytes: int64(len(img.Pix)),
	}
	if got := p.Status(); got.LastFrame != want || got.PresentedFrames != 1 || got.Frames != 1 {
		t.Fatalf("diagnostics = %+v, want frame %+v", got, want)
	}
	now = now.Add(time.Hour)
	p.Update(presented)
	if p.Status().LastFrame != want || p.Status().PresentedFrames != 1 {
		t.Fatal("duplicate presentation changed diagnostics")
	}
	// Status returns a value, not mutable shared instrumentation.
	copy := p.Status()
	copy.LastFrame.Bytes = -1
	if p.Status().LastFrame != want {
		t.Fatal("Status leaked mutable diagnostics")
	}
}

func TestDiagnosticsAbandonUnconfirmedSample(t *testing.T) {
	for _, event := range []string{"resize", "stale", "picture error", "newer presentation", "loop error", "close"} {
		t.Run(event, func(t *testing.T) {
			p := testProducer(t)
			now := time.Unix(100, 0)
			p.now = func() time.Time { return now }
			p.Init()
			p.Update(readyFrame(p))
			now = now.Add(time.Millisecond)
			p.Update(ntpicture.PresentedMsg{Owner: p.pic, Generation: p.submitted})
			previous := p.Status().LastFrame
			p.Update(readyFrame(p))
			pending := p.submitted
			switch event {
			case "resize":
				p.pic.SetSize(20, 8)
				p.Update(nil)
			case "stale":
				p.Update(ntpicture.StaleFrameMsg{Owner: p.pic})
			case "picture error":
				p.Update(ntpicture.ErrorMsg{Owner: p.pic, Generation: pending, Err: errors.New("output failed")})
			case "newer presentation":
				p.Update(ntpicture.PresentedMsg{Owner: p.pic, Generation: pending + 1})
			case "loop error":
				p.Update(loopStoppedMsg{owner: p, err: errors.New("loop failed")})
			case "close":
				p.Close()()
			}
			now = now.Add(time.Hour)
			p.Update(ntpicture.PresentedMsg{Owner: p.pic, Generation: pending})
			if p.Status().LastFrame != previous || p.Status().PresentedFrames != 1 {
				t.Fatal("abandoned submission produced a sample or erased previous diagnostics")
			}
			if p.pendingFrame.Generation != 0 || !p.submittedAt.IsZero() {
				t.Fatal("pending measurement survived invalidation")
			}
			if p.Status().DiscardedRenders != 0 {
				t.Fatal("a submitted frame was counted as a discarded render")
			}
		})
	}
}

func TestDiagnosticsCountObsoleteRenderOnce(t *testing.T) {
	p := testProducer(t)
	p.Init()
	old := readyFrame(p)
	// Cancellation can skip readback entirely and must still count when drained.
	old.img, old.err = nil, context.Canceled
	for size := 13; size < 20; size++ {
		p.pic.SetSize(size, 7)
		p.Update(nil)
	}
	if p.Status().DiscardedRenders != 0 {
		t.Fatal("counted invalidation events instead of discarded results")
	}
	foreign := old
	foreign.owner = nil
	p.Update(foreign)
	p.Update(old)
	p.Update(old)
	if got := p.Status(); got.DiscardedRenders != 1 || got.LastFrame.Generation != 0 || got.Frames != 0 {
		t.Fatalf("wrong obsolete-render diagnostics: %+v", got)
	}
	current := readyFrame(p)
	p.Close()()
	p.Update(current)
	if p.Status().DiscardedRenders != 1 {
		t.Fatal("shutdown counted as a geometry discard")
	}
}
