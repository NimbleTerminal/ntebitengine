package ntebitengine

import (
	"context"
	"errors"
	"fmt"
	"image"
	"sync"
	"sync/atomic"

	"github.com/hajimehoshi/ebiten/v2"
	"nimbleterminal.dev/ntpicture"
)

// RunGame is process-global and may never be restarted, even after it exits.
var loopClaimed atomic.Bool

type request struct {
	ctx        context.Context
	generation uint64
	geometry   ntpicture.RasterGeometry
	reply      chan renderResult
}

type renderResult struct {
	img *image.RGBA
	err error
}

// gameLoop contains only immutable configuration, channels, and synchronized
// lifecycle state; it never reads the producer's event-loop state or Picture.
type gameLoop struct {
	draw     DrawFunc
	tps      int
	hide     bool
	requests chan *request
	stop     chan struct{}
	done     chan struct{}
	mu       sync.Mutex
	started  bool
	closed   bool
	err      error // written before done closes; read only after receiving done
	runGame  func(ebiten.Game) error
}

func newGameLoop(draw DrawFunc, opts Options) *gameLoop {
	tps := opts.TPS
	if tps <= 0 {
		tps = 60
	}
	hide := opts.HideWindow == nil || *opts.HideWindow
	return &gameLoop{draw: draw, tps: tps, hide: hide,
		requests: make(chan *request, 1), stop: make(chan struct{}), done: make(chan struct{}),
		runGame: runEbitengine}
}

func runEbitengine(game ebiten.Game) error {
	if !loopClaimed.CompareAndSwap(false, true) {
		return errors.New("ntebitengine: only one Ebitengine loop may run per process; restarting is unsupported")
	}
	g := game.(*frameGame)
	ebiten.SetWindowVisible(!g.loop.hide)
	ebiten.SetTPS(g.loop.tps)
	ebiten.SetRunnableOnUnfocused(true)
	return ebiten.RunGame(game)
}

func (l *gameLoop) run() error {
	l.mu.Lock()
	if l.closed {
		l.mu.Unlock()
		return nil
	}
	if l.started {
		l.mu.Unlock()
		return errors.New("ntebitengine: loop already started")
	}
	l.started = true
	l.mu.Unlock()
	var err error
	if l.draw == nil {
		err = errors.New("ntebitengine: draw must be non-nil")
	} else {
		err = l.runGame(&frameGame{loop: l})
	}
	l.err = err
	close(l.done)
	return err
}

func (l *gameLoop) close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return
	}
	l.closed = true
	close(l.stop)
	if !l.started {
		close(l.done)
	}
}

// All GPU work happens in Update so TPS is the request pickup rate. The hidden
// window's screen is a tiny unused surface; the framebuffer has exact Pixels.
type frameGame struct {
	loop *gameLoop
	dst  *ebiten.Image
}

func (g *frameGame) Update() error {
	select {
	case <-g.loop.stop:
		if g.dst != nil {
			g.dst.Dispose()
			g.dst = nil
		}
		return ebiten.Termination
	default:
	}
	select {
	case r := <-g.loop.requests:
		r.reply <- g.render(r)
	default:
	}
	return nil
}

func (g *frameGame) Draw(*ebiten.Image)         {}
func (g *frameGame) Layout(int, int) (int, int) { return 1, 1 }

func (g *frameGame) render(r *request) renderResult {
	if err := r.ctx.Err(); err != nil {
		return renderResult{err: err}
	}
	size := r.geometry.Pixels
	limit := ebiten.MaxImageSize()
	if size.X <= 0 || size.Y <= 0 || size.X > limit || size.Y > limit || size.X > ntpicture.MaxImagePixels/size.Y {
		return renderResult{err: fmt.Errorf("ntebitengine: unsupported framebuffer size %v (GPU dimension limit %d)", size, limit)}
	}
	if g.dst == nil || g.dst.Bounds().Size() != size {
		if g.dst != nil {
			g.dst.Dispose()
		}
		g.dst = ebiten.NewImage(size.X, size.Y)
	}
	g.dst.Clear()
	if err := g.loop.draw(g.dst, r.geometry); err != nil {
		return renderResult{err: err}
	}
	if err := r.ctx.Err(); err != nil {
		return renderResult{err: err}
	}
	select {
	case <-g.loop.stop:
		return renderResult{err: context.Canceled}
	default:
	}
	// ReadPixels returns premultiplied RGBA, matching image.RGBA. Each frame
	// owns fresh CPU pixels that are never reused after the immutable handoff.
	img := image.NewRGBA(image.Rectangle{Max: size})
	g.dst.ReadPixels(img.Pix)
	return renderResult{img: img}
}
