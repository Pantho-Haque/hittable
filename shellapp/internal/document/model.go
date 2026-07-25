package document

import "sync"

type ViewMode int

const (
	ViewRunner ViewMode = iota
	ViewText
)

type FileKind int

const (
	KindHit FileKind = iota
	KindEnv
	KindMarkdown
	KindGeneric
)

type DocumentModel struct {
	mu           sync.Mutex
	Path         string
	Kind         FileKind
	Content      string
	RawContent   []byte
	ViewMode     ViewMode
	HitContent   interface{}
	Generation   uint64
	GenCounter   uint64
	LastFlushed  uint64
}

func NewDocumentModel(path string, kind FileKind, content string, raw []byte) *DocumentModel {
	return &DocumentModel{
		Path:       path,
		Kind:       kind,
		Content:    content,
		RawContent: raw,
		ViewMode:   ViewRunner,
		Generation: 1,
		GenCounter: 1,
	}
}

func (d *DocumentModel) Lock()   { d.mu.Lock() }
func (d *DocumentModel) Unlock() { d.mu.Unlock() }

func (d *DocumentModel) IncGeneration() uint64 {
	d.GenCounter++
	d.Generation = d.GenCounter
	return d.Generation
}

func (d *DocumentModel) SetContent(content string) {
	d.Content = content
}

func (d *DocumentModel) GetContent() string {
	return d.Content
}

func (d *DocumentModel) GetGeneration() uint64 {
	return d.Generation
}

func (d *DocumentModel) SetLastFlushed(gen uint64) {
	d.LastFlushed = gen
}

func (d *DocumentModel) GetLastFlushed() uint64 {
	return d.LastFlushed
}
