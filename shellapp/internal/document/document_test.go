package document

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestWriteQueueRaceTwoRapidUpdates(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "wq-race-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "test.txt")

	wq := NewWriteQueue()
	wq.delay = 50 * time.Millisecond

	wq.Update(testFile, "first content", 1)

	time.Sleep(5 * time.Millisecond)

	wq.Update(testFile, "second content", 2)

	time.Sleep(200 * time.Millisecond)

	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	if string(content) != "second content" {
		t.Errorf("expected 'second content', got %q", string(content))
	}
}

func TestWriteQueueConcurrentUpdates(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "wq-concurrent-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "test.txt")

	wq := NewWriteQueue()
	wq.delay = 30 * time.Millisecond

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			content := "content_" + string(rune('A'+n%26))
			wq.Update(testFile, content, uint64(n+1))
		}(i)
	}
	wg.Wait()

	time.Sleep(200 * time.Millisecond)

	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	if len(content) == 0 {
		t.Error("file is empty after concurrent updates")
	}
}

func TestDocumentModelThreadSafety(t *testing.T) {
	doc := NewDocumentModel("/test/file.txt", KindGeneric, "initial", []byte("initial"))

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			doc.Lock()
			doc.SetContent("content_" + string(rune('A'+n%26)))
			doc.IncGeneration()
			doc.Unlock()
		}(i)
	}
	wg.Wait()

	if doc.GetGeneration() != 101 {
		t.Errorf("expected generation 101, got %d", doc.GetGeneration())
	}
}
