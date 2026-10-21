// Package ntebitengine renders Ebitengine frames into an ntpicture.Picture.
// Configure the picture with StableID and FitFill, and forward every Bubble Tea
// message to Picture.Update before Producer.Update. See Producer.Run for the
// portable main-thread startup path.
package ntebitengine

import (
	"context"
	"errors"
	"image"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/hajimehoshi/ebiten/v2"
	"nimbleterminal.dev/ntpicture"
)

// DrawFunc draws one frame into dst, already cleared and sized to geo.Pixels.
// It runs on Ebitengine's goroutine, not the Bubble Tea event loop. It must not
// retain dst or its pixels, dispose or resize dst, or access the Picture.
// Return promptly: cancellation cannot interrupt a callback already running.
type DrawFunc func(dst *ebiten.Image, geo ntpicture.RasterGeometry) error

type Options struct {
	// TPS controls how quickly the loop picks up requests; nonpositive means 60.
	// Presentation, rather than TPS, controls when another frame is requested.
	TPS int
	// HideWindow defaults to true. This still requires a working graphics driver.
	HideWindow *bool
	// ManualRun disables Init's background loop startup. Set this and call Run
	// from main for platforms requiring Ebitengine on the main OS thread.
	ManualRun bool
}

// FrameStats describes one producer frame confirmed by an exact matching
// PresentedMsg. Durations are wall-clock measurements, not GPU execution time.
// The stages omit request pickup, surface/image allocation, clearing, and result
// delivery; their sum is not end-to-end frame latency.
type FrameStats struct {
	Generation   uint64 // producer generation, not Picture generation; zero means no sample
	Geometry     ntpicture.RasterGeometry
	DrawTime     time.Duration // time inside DrawFunc, including any blocking work
	ReadbackTime time.Duration // time inside ReadPixels, including GPU synchronization
	PresentTime  time.Duration // submission call through delivery of PresentedMsg to Update
	Bytes        int64         // raw immutable RGBA buffer bytes, not encoded/transport bytes
}

// Status is a snapshot owned by the Bubble Tea event loop. Diagnostics are
// cumulative since New; LastFrame remains available across resize/error/Close.
type Status struct {
	Geometry             ntpicture.RasterGeometry
	Generation           uint64     // producer request generation, not a Picture generation
	Frames               uint64     // frames submitted to Picture
	PresentedFrames      uint64     // exact matching confirmations of producer submissions
	DiscardedRenders     uint64     // obsolete results drained after geometry changes (even if canceled before drawing)
	LastFrame            FrameStats // most recently confirmed producer frame; zero until one is confirmed
	InFlight             bool
	AwaitingPresentation bool
	Closed               bool
	Err                  error
}

// ErrorMsg reports a drawing, presentation, or loop error. Drawing and picture
// errors pause production until geometry changes; loop errors are terminal.
// Forward this message to Update too; Status already contains Err when emitted.
type ErrorMsg struct {
	Owner *Producer
	Err   error
}

// Producer owns one Ebitengine loop and feeds a Picture. Do not copy it.
// All methods except Run belong to the Bubble Tea event loop. After Program.Run
// returns, the same owner may call Close for cleanup. Only one Producer may run
// per process, and it cannot restart after Close (an Ebitengine limitation).
type Producer struct {
	pic          *ntpicture.Picture
	loop         *gameLoop
	manual       bool
	initialized  bool
	stopped      bool
	status       Status
	active       *request
	cancel       context.CancelFunc
	submitted    uint64
	submittedAt  time.Time
	pendingFrame FrameStats
	now          func() time.Time // event-loop clock; replaceable in deterministic tests
}

// New binds draw to pic. Construct pic with StableID: true and without
// ClearOnReplace; Init sets FitFill. The producer is the picture's sole image
// source. The host still owns Picture.Init, Update, layout, View, and Close.
// Nil pic or draw is reported as ErrorMsg by Init.
func New(pic *ntpicture.Picture, draw DrawFunc, opts Options) *Producer {
	return &Producer{pic: pic, loop: newGameLoop(draw, opts), manual: opts.ManualRun, now: time.Now}
}

// Init starts the background loop unless ManualRun is set, and requests the
// first frame once the picture has positive pixel dimensions. It is idempotent.
// Run the returned command. Background RunGame is not portable; prefer Run.
func (p *Producer) Init() tea.Cmd {
	if p.initialized || p.status.Closed {
		return nil
	}
	p.initialized = true
	if p.pic == nil || p.loop.draw == nil {
		p.stopped = true
		p.loop.close()
		return p.fail(errors.New("ntebitengine: picture and draw must be non-nil"))
	}
	if p.pic.Closed() {
		return p.Close()
	}
	p.status.Geometry = p.pic.RasterGeometry()
	fit := p.pic.SetFit(ntpicture.FitFill)
	loop, manual := p.loop, p.manual
	watch := func() tea.Msg {
		if !manual {
			go loop.run()
		}
		<-loop.done
		return loopStoppedMsg{owner: p, err: loop.err}
	}
	return tea.Batch(fit, watch, p.start())
}

// Run runs Ebitengine on the calling thread until Close or a loop error.
// Set Options.ManualRun, launch Bubble Tea in a goroutine, then call Run from
// main. Run may precede Init; it only accesses synchronized loop state. The Tea
// goroutine must call Close even when Program.Run returns an error.
func (p *Producer) Run() error {
	if !p.manual {
		return errors.New("ntebitengine: Run requires Options.ManualRun")
	}
	return p.loop.run()
}

// Update receives every message, after host layout changes and Picture.Update.
// It does not call Picture.Update itself. Viewport-only changes leave the raster
// geometry unchanged and do not invalidate or request a frame.
func (p *Producer) Update(msg tea.Msg) tea.Cmd {
	if !p.initialized || p.status.Closed || p.stopped {
		return nil
	}
	if p.pic.Closed() {
		return p.Close()
	}
	if geo := p.pic.RasterGeometry(); geo != p.status.Geometry {
		p.status.Geometry = geo
		p.status.Generation++
		if p.cancel != nil {
			p.cancel()
		}
		p.status.AwaitingPresentation = false
		p.forgetPresentation()
		p.status.Err = nil
	}
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case frameMsg:
		if msg.owner != p || msg.request != p.active {
			break
		}
		p.cancel()
		p.cancel, p.active = nil, nil
		p.status.InFlight = false
		if msg.request.generation != p.status.Generation {
			p.status.DiscardedRenders++
			break // Drain obsolete work before opening the render slot again.
		}
		if p.status.Err != nil {
			break // A picture error may have arrived while this frame rendered.
		}
		if msg.err != nil {
			cmd = p.fail(msg.err)
			break
		}
		before := p.pic.Status().Generation
		submittedAt := p.now()
		cmd = p.pic.SetImageImmutableFor(msg.img, msg.request.geometry)
		if generation := p.pic.Status().Generation; generation != before {
			p.submitted = generation
			p.status.Frames++
			if msg.img != nil {
				p.submittedAt = submittedAt
				p.pendingFrame = FrameStats{
					Generation: msg.request.generation, Geometry: msg.request.geometry,
					DrawTime: msg.drawTime, ReadbackTime: msg.readbackTime,
					Bytes: int64(len(msg.img.Pix)),
				}
			}
		}
		// Even a rejected submission waits for its StaleFrameMsg.
		p.status.AwaitingPresentation = true
	case ntpicture.PresentedMsg:
		if msg.Owner == p.pic && p.status.AwaitingPresentation && msg.Generation >= p.submitted {
			// Newer Picture commits can release pacing, but cannot be timed as
			// this submission. Ignore duplicate, superseded, and foreign samples.
			if msg.Generation == p.submitted && p.pendingFrame.Generation != 0 {
				p.pendingFrame.PresentTime = p.now().Sub(p.submittedAt)
				p.status.LastFrame = p.pendingFrame
				p.status.PresentedFrames++
			}
			p.forgetPresentation()
			p.status.AwaitingPresentation = false
		}
	case ntpicture.StaleFrameMsg:
		if msg.Owner == p.pic {
			p.forgetPresentation()
			p.status.AwaitingPresentation = false
		}
	case ntpicture.ErrorMsg:
		if msg.Owner == p.pic && msg.Generation >= p.submitted {
			cmd = p.fail(msg.Err)
		}
	case loopStoppedMsg:
		if msg.owner == p {
			p.stopped = true
			p.status.InFlight = false
			p.status.AwaitingPresentation = false
			if p.cancel != nil {
				p.cancel()
			}
			if msg.err == nil {
				msg.err = errors.New("ntebitengine: Ebitengine loop exited")
			}
			cmd = p.fail(msg.err)
		}
	}
	return tea.Batch(cmd, p.start())
}

// Close cancels the active request and stops the loop immediately; its command
// waits for the loop to exit. Use tea.Sequence(p.Close(), pic.Close(), tea.Quit).
// A running DrawFunc must return before shutdown completes. Close is idempotent
// and does not close the Picture. The producer cannot be restarted.
func (p *Producer) Close() tea.Cmd {
	if !p.status.Closed {
		p.status.Closed = true
		p.status.Generation++
		p.status.AwaitingPresentation = false
		p.forgetPresentation()
		p.status.InFlight = false
		if p.cancel != nil {
			p.cancel()
		}
		p.loop.close()
	}
	loop := p.loop
	return func() tea.Msg { <-loop.done; return nil }
}

func (p *Producer) Status() Status { return p.status }

func (p *Producer) fail(err error) tea.Cmd {
	p.status.Err = err
	p.status.AwaitingPresentation = false
	p.forgetPresentation()
	return func() tea.Msg { return ErrorMsg{Owner: p, Err: err} }
}

func (p *Producer) forgetPresentation() {
	p.submittedAt = time.Time{}
	p.pendingFrame = FrameStats{}
}

func (p *Producer) start() tea.Cmd {
	s := &p.status
	if !p.initialized || p.stopped || s.Closed || s.InFlight || s.AwaitingPresentation || s.Err != nil || p.pic.Closed() {
		return nil
	}
	geo := p.pic.RasterGeometry()
	if geo.Pixels.X <= 0 || geo.Pixels.Y <= 0 {
		return nil
	}
	s.Generation++
	ctx, cancel := context.WithCancel(context.Background())
	r := &request{ctx: ctx, generation: s.Generation, geometry: geo, reply: make(chan renderResult, 1)}
	p.active, p.cancel, s.InFlight = r, cancel, true
	loop := p.loop
	return func() tea.Msg {
		result := renderResult{err: context.Canceled}
		select {
		case loop.requests <- r:
			// Do not select ctx.Done: a canceled callback still occupies the slot
			// until the game acknowledges it. Close may abandon the slot entirely.
			select {
			case result = <-r.reply:
			case <-loop.stop:
			case <-loop.done:
				return loopStoppedMsg{owner: p, err: loop.err}
			}
		case <-loop.stop:
		case <-loop.done:
			return loopStoppedMsg{owner: p, err: loop.err}
		}
		return frameMsg{owner: p, request: r, img: result.img, err: result.err,
			drawTime: result.drawTime, readbackTime: result.readbackTime}
	}
}

type frameMsg struct {
	owner                  *Producer
	request                *request
	img                    *image.RGBA
	err                    error
	drawTime, readbackTime time.Duration
}

type loopStoppedMsg struct {
	owner *Producer
	err   error
}
