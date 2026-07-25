package document

import "sync"

type Store struct {
	mu        sync.RWMutex
	documents map[string]*DocumentModel
}

func NewStore() *Store {
	return &Store{
		documents: make(map[string]*DocumentModel),
	}
}

func (s *Store) Get(path string) *DocumentModel {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.documents[path]
}

func (s *Store) GetOrCreate(path string, kind FileKind, content string, raw []byte) *DocumentModel {
	s.mu.Lock()
	defer s.mu.Unlock()
	if doc, ok := s.documents[path]; ok {
		return doc
	}
	doc := NewDocumentModel(path, kind, content, raw)
	s.documents[path] = doc
	return doc
}

func (s *Store) Set(path string, doc *DocumentModel) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.documents[path] = doc
}

func (s *Store) Has(path string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.documents[path]
	return ok
}

func (s *Store) Delete(path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.documents, path)
}
