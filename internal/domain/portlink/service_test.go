package portlink

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"
)

type memoryStore struct {
	items []Link
}

func (s *memoryStore) ListByServer(_ context.Context, serverID string) ([]Link, error) {
	var items []Link
	for _, item := range s.items {
		if item.ServerID == serverID {
			items = append(items, item)
		}
	}
	return items, nil
}

func (s *memoryStore) GetByServerPort(_ context.Context, serverID string, port int) (Link, error) {
	for _, item := range s.items {
		if item.ServerID == serverID && item.Port == port {
			return item, nil
		}
	}
	return Link{}, ErrNotFound
}

func (s *memoryStore) Save(_ context.Context, item Link) error {
	for index, existing := range s.items {
		if existing.ID == item.ID {
			s.items[index] = item
			return nil
		}
	}
	s.items = append(s.items, item)
	return nil
}

func (s *memoryStore) Delete(_ context.Context, id string) error {
	for index, item := range s.items {
		if item.ID == id {
			s.items = append(s.items[:index], s.items[index+1:]...)
			return nil
		}
	}
	return nil
}

func TestServiceSaveCreatesAndUpdatesOneLinkPerServerPort(t *testing.T) {
	store := &memoryStore{}
	service := NewService(store)

	created, err := service.Save(context.Background(), Link{
		ServerID: "server-1",
		Port:     3000,
		Name:     "  Admin app  ",
		Scheme:   SchemeHTTP,
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.ID == "" || created.Name != "Admin app" || created.CreatedAt.IsZero() {
		t.Fatalf("created link = %#v", created)
	}

	updated, err := service.Save(context.Background(), Link{
		ServerID:       "server-1",
		Port:           3000,
		Name:           "Dashboard",
		Scheme:         SchemeHTTPS,
		FaviconDataURL: "data:image/png;base64,aWNvbg==",
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.ID != created.ID || !updated.CreatedAt.Equal(created.CreatedAt) {
		t.Fatalf("updated identity = %#v, created = %#v", updated, created)
	}
	if updated.Name != "Dashboard" || updated.Scheme != SchemeHTTPS {
		t.Fatalf("updated link = %#v", updated)
	}
	if len(store.items) != 1 {
		t.Fatalf("stored links = %#v", store.items)
	}
}

func TestLinkValidateRejectsUnsafeOrOversizedValues(t *testing.T) {
	valid := Link{
		ServerID: "server-1",
		Port:     8080,
		Name:     "Dashboard",
		Scheme:   SchemeHTTP,
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid link: %v", err)
	}

	tests := []struct {
		name string
		edit func(*Link)
		want string
	}{
		{name: "port", edit: func(link *Link) { link.Port = 0 }, want: "port"},
		{name: "server", edit: func(link *Link) { link.ServerID = " " }, want: "server"},
		{name: "name", edit: func(link *Link) { link.Name = " " }, want: "name"},
		{name: "scheme", edit: func(link *Link) { link.Scheme = "file" }, want: "scheme"},
		{name: "favicon type", edit: func(link *Link) { link.FaviconDataURL = "data:text/html;base64,eA==" }, want: "favicon"},
		{name: "favicon size", edit: func(link *Link) {
			link.FaviconDataURL = "data:image/png;base64," + strings.Repeat("a", MaxFaviconDataURLBytes)
		}, want: "favicon"},
		{name: "name size", edit: func(link *Link) { link.Name = strings.Repeat("a", maxNameBytes+1) }, want: "name"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := valid
			test.edit(&input)
			if err := input.Validate(); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Validate() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestServicePreservesCallerTimestampOnExistingID(t *testing.T) {
	createdAt := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	store := &memoryStore{items: []Link{{
		ID:        "link-1",
		ServerID:  "server-1",
		Port:      3000,
		Name:      "App",
		Scheme:    SchemeHTTP,
		CreatedAt: createdAt,
		UpdatedAt: createdAt,
	}}}
	service := NewService(store)

	updated, err := service.Save(context.Background(), Link{
		ID:       "link-1",
		ServerID: "server-1",
		Port:     3000,
		Name:     "Renamed",
		Scheme:   SchemeHTTP,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !updated.CreatedAt.Equal(createdAt) {
		t.Fatalf("created at = %v, want %v", updated.CreatedAt, createdAt)
	}
}

type concurrentCreateStore struct {
	mu           sync.Mutex
	firstReads   int
	firstReadsOK chan struct{}
	releaseReads chan struct{}
	item         Link
}

func newConcurrentCreateStore() *concurrentCreateStore {
	return &concurrentCreateStore{
		firstReadsOK: make(chan struct{}),
		releaseReads: make(chan struct{}),
	}
}

func (store *concurrentCreateStore) ListByServer(context.Context, string) ([]Link, error) {
	return nil, nil
}

func (store *concurrentCreateStore) GetByServerPort(context.Context, string, int) (Link, error) {
	store.mu.Lock()
	store.firstReads++
	readNumber := store.firstReads
	if readNumber == 2 {
		close(store.firstReadsOK)
	}
	if readNumber > 2 {
		item := store.item
		store.mu.Unlock()
		return item, nil
	}
	store.mu.Unlock()
	<-store.releaseReads
	return Link{}, ErrNotFound
}

func (store *concurrentCreateStore) Save(_ context.Context, item Link) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.item.ID == "" {
		store.item = item
		return nil
	}
	item.ID = store.item.ID
	item.CreatedAt = store.item.CreatedAt
	store.item = item
	return nil
}

func (store *concurrentCreateStore) Delete(context.Context, string) error {
	return nil
}

func TestServiceConcurrentCreatesReturnCanonicalStoredIdentity(t *testing.T) {
	store := newConcurrentCreateStore()
	service := NewService(store)
	results := make(chan Link, 2)
	errors := make(chan error, 2)

	for _, name := range []string{"First", "Second"} {
		name := name
		go func() {
			link, err := service.Save(context.Background(), Link{
				ServerID: "server-1",
				Port:     3000,
				Name:     name,
				Scheme:   SchemeHTTP,
			})
			results <- link
			errors <- err
		}()
	}

	<-store.firstReadsOK
	close(store.releaseReads)

	first := <-results
	second := <-results
	if err := <-errors; err != nil {
		t.Fatal(err)
	}
	if err := <-errors; err != nil {
		t.Fatal(err)
	}
	if first.ID == "" || first.ID != second.ID || first.ID != store.item.ID {
		t.Fatalf("returned IDs = %q and %q, stored ID = %q", first.ID, second.ID, store.item.ID)
	}
	if !first.CreatedAt.Equal(store.item.CreatedAt) || !second.CreatedAt.Equal(store.item.CreatedAt) {
		t.Fatalf("returned creation times = %v and %v, stored = %v", first.CreatedAt, second.CreatedAt, store.item.CreatedAt)
	}
}

func TestServiceCreatesItsOwnIdentityForNewLinks(t *testing.T) {
	store := &memoryStore{}
	service := NewService(store)

	created, err := service.Save(context.Background(), Link{
		ID:       "caller-supplied",
		ServerID: "server-1",
		Port:     3000,
		Name:     "App",
		Scheme:   SchemeHTTP,
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.ID == "" || created.ID == "caller-supplied" {
		t.Fatalf("created ID = %q, want a service-generated ID", created.ID)
	}
}
