package terminal

import (
	"time"

	"github.com/gdamore/tcell/v2"
)

// eventBuffer bounds the events queued between the reader and the loop. When
// it is full the reader stops polling, back-pressuring tcell's own bounded
// queue, so input floods cannot grow memory.
const eventBuffer = 64

// eventReader forwards backend events to the loop goroutine, which is the
// only owner of engine and game state.
//
// It replaces tcell's ChannelEvents because of a shutdown hazard in tcell
// v2.13: the backend's input goroutine posts to its internal queue with a
// blocking send, and Fini waits for that goroutine. ChannelEvents stops
// reading as soon as Fini begins, so a full queue at quit time could hang
// Fini. This reader keeps draining until Fini has returned.
type eventReader struct {
	events   chan tcell.Event
	stopping chan struct{}
	finished chan struct{}
	done     chan struct{}
}

// startReader polls with poll, which must block until an event arrives and
// return nil once the backend is finalizing (tcell's PollEvent contract).
func startReader(poll func() tcell.Event) *eventReader {
	r := &eventReader{
		events:   make(chan tcell.Event, eventBuffer),
		stopping: make(chan struct{}),
		finished: make(chan struct{}),
		done:     make(chan struct{}),
	}
	go r.run(poll)
	return r
}

func (r *eventReader) run(poll func() tcell.Event) {
	defer close(r.done)
	for {
		ev := poll()
		if ev == nil {
			// The backend is finalizing. Stragglers may still be posted
			// until Fini returns; check back briefly rather than spin.
			select {
			case <-r.finished:
				return
			case <-time.After(time.Millisecond):
				continue
			}
		}
		select {
		case r.events <- ev:
		case <-r.stopping: // discard
		}
	}
}

// stop ends forwarding, runs fini (which must make poll return nil) while
// discarding backend events, and waits for the reader goroutine to exit. It
// must be called exactly once.
func (r *eventReader) stop(fini func()) {
	close(r.stopping)
	defer func() {
		close(r.finished)
		<-r.done
	}()
	fini()
}
