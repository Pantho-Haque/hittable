package document

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWriteQueueCancelDropsPendingWrite(t *testing.T) {
	p := filepath.Join(t.TempDir(), "deleted.txt")
	wq := NewWriteQueue()
	wq.delay = 20 * time.Millisecond
	wq.Update(p, "late write", 2)
	wq.Cancel(p) // the file was deleted in the explorer
	time.Sleep(80 * time.Millisecond)
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Error("a cancelled write recreated the deleted file")
	}
	wq.Cancel(p) // cancelling twice is harmless
}

func TestWriteQueueFlushNowWritesOnceAndOlderIsIgnored(t *testing.T) {
	dir := t.TempDir()
	a, b := filepath.Join(dir, "a.txt"), filepath.Join(dir, "b.txt")
	wq := NewWriteQueue()
	wq.delay = time.Hour // only FlushNow may write
	wq.Update(a, "a3", 3)
	wq.Update(a, "a2", 2) // older generation: dropped
	wq.Update(b, "b1", 2)
	wq.FlushNow()
	if got, _ := os.ReadFile(a); string(got) != "a3" {
		t.Errorf("a = %q", got)
	}
	if got, _ := os.ReadFile(b); string(got) != "b1" {
		t.Errorf("b = %q", got)
	}
	// After a flush the queue is empty: an external edit is not overwritten
	// by a second flush.
	os.WriteFile(a, []byte("external"), 0o644)
	wq.FlushNow()
	if got, _ := os.ReadFile(a); string(got) != "external" {
		t.Errorf("second flush rewrote a: %q", got)
	}
	// A write into a vanished directory fails silently, not with a panic.
	wq.Update(filepath.Join(dir, "gone", "c.txt"), "c", 1)
	wq.FlushNow()
}

func TestStoreLifecycle(t *testing.T) {
	s := NewStore()
	d := s.GetOrCreate("/p", KindHit, "one", nil)
	if s.GetOrCreate("/p", KindGeneric, "two", nil) != d || d.Content != "one" {
		t.Error("GetOrCreate must reuse the open document")
	}
	if !s.Has("/p") || s.Get("/missing") != nil || len(s.Paths()) != 1 {
		t.Error("Has / Get / Paths")
	}
	d.Lock()
	g := d.IncGeneration()
	d.SetLastFlushed(g)
	d.Unlock()
	if g != 2 || d.GetGeneration() != 2 || d.GetLastFlushed() != 2 {
		t.Errorf("generation bookkeeping: %d %d %d", g, d.GetGeneration(), d.GetLastFlushed())
	}
	s.Delete("/p")
	if s.Has("/p") {
		t.Error("Delete")
	}
	if fresh := s.GetOrCreate("/p", KindHit, "new", nil); fresh == d || fresh.Generation != 1 {
		t.Error("re-opening a deleted path should start a fresh document")
	}
}
