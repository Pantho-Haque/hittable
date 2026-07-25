package document

import (
	"os"
	"sync"
	"time"
)

type writeRequest struct {
	path       string
	content    string
	generation uint64
}

type WriteQueue struct {
	mu       sync.Mutex
	queue    map[string]writeRequest
	timers   map[string]*time.Timer
	delay    time.Duration
	inflight map[string]bool
}

func NewWriteQueue() *WriteQueue {
	return &WriteQueue{
		queue:    make(map[string]writeRequest),
		timers:   make(map[string]*time.Timer),
		delay:    200 * time.Millisecond,
		inflight: make(map[string]bool),
	}
}

func (w *WriteQueue) Enqueue(path, content string, generation uint64) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.queue[path] = writeRequest{
		path:       path,
		content:    content,
		generation: generation,
	}

	if t, ok := w.timers[path]; ok {
		t.Stop()
	}

	w.timers[path] = time.AfterFunc(w.delay, func() {
		w.flush(path)
	})
}

func (w *WriteQueue) flush(path string) {
	w.mu.Lock()
	req, ok := w.queue[path]
	if !ok {
		w.mu.Unlock()
		return
	}
	delete(w.queue, path)
	delete(w.timers, path)
	w.mu.Unlock()

	_ = os.WriteFile(path, []byte(req.content), 0o644)

	w.mu.Lock()
	w.inflight[path] = false
	w.mu.Unlock()
}

func (w *WriteQueue) FlushNow() {
	w.mu.Lock()
	paths := make([]string, 0, len(w.queue))
	for p := range w.queue {
		paths = append(paths, p)
	}
	w.mu.Unlock()

	for _, p := range paths {
		w.mu.Lock()
		req, ok := w.queue[p]
		if ok {
			delete(w.queue, p)
			if t, ok := w.timers[p]; ok {
				t.Stop()
				delete(w.timers, p)
			}
		}
		w.mu.Unlock()
		if ok {
			_ = os.WriteFile(p, []byte(req.content), 0o644)
		}
	}
}

func (w *WriteQueue) Update(path, content string, generation uint64) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if existing, ok := w.queue[path]; ok {
		if generation <= existing.generation {
			return
		}
	}

	w.queue[path] = writeRequest{
		path:       path,
		content:    content,
		generation: generation,
	}

	if t, ok := w.timers[path]; ok {
		t.Stop()
	}

	w.timers[path] = time.AfterFunc(w.delay, func() {
		w.mu.Lock()
		req, ok := w.queue[path]
		if !ok {
			w.mu.Unlock()
			return
		}
		if req.generation != generation {
			w.mu.Unlock()
			return
		}
		delete(w.queue, path)
		delete(w.timers, path)
		w.mu.Unlock()

		_ = os.WriteFile(path, []byte(req.content), 0o644)
	})
}
