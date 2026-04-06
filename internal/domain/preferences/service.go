package preferences

import (
	"context"
	"time"
)

type Store interface {
	Load(ctx context.Context) (UserPreference, error)
	Save(ctx context.Context, pref UserPreference) error
}

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

func (s *Service) Load(ctx context.Context) (UserPreference, error) {
	return s.store.Load(ctx)
}

func (s *Service) Save(ctx context.Context, pref UserPreference) (UserPreference, error) {
	if pref.Theme == "" {
		pref.Theme = ThemeDark
	}
	pref.UpdatedAt = time.Now().UTC()
	if err := pref.Validate(); err != nil {
		return UserPreference{}, err
	}
	if err := s.store.Save(ctx, pref); err != nil {
		return UserPreference{}, err
	}
	return pref, nil
}
