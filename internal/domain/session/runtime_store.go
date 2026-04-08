package session

import "sync"

type Runner interface {
	Start() error
	Stop() error
	BoundPort() int
}

type runtimeEntry struct {
	state      RuntimeSession
	runner     Runner
	passphrase string
	token      string
}

type RuntimeStore struct {
	mu       sync.RWMutex
	sessions map[string]runtimeEntry
}

func NewRuntimeStore() *RuntimeStore {
	return &RuntimeStore{sessions: make(map[string]runtimeEntry)}
}

func (s *RuntimeStore) Get(id string) (RuntimeSession, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.sessions[id]
	return entry.state, ok
}

func (s *RuntimeStore) Runner(id string) (Runner, string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.sessions[id]
	return entry.runner, entry.passphrase, ok
}

func (s *RuntimeStore) Token(id string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.sessions[id]
	return entry.token, ok
}

func (s *RuntimeStore) Set(state RuntimeSession, runner Runner, passphrase string) {
	s.SetWithToken(state, runner, passphrase, "")
}

func (s *RuntimeStore) SetWithToken(state RuntimeSession, runner Runner, passphrase string, token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[state.ConfigurationID] = runtimeEntry{state: state, runner: runner, passphrase: passphrase, token: token}
}

func (s *RuntimeStore) Delete(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, id)
}

func (s *RuntimeStore) List() []RuntimeSession {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]RuntimeSession, 0, len(s.sessions))
	for _, entry := range s.sessions {
		items = append(items, entry.state)
	}
	return items
}
