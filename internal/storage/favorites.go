package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Favorite represents a saved YouTube URL
type Favorite struct {
	Name      string    `json:"name"`
	URL       string    `json:"url"`
	CreatedAt time.Time `json:"created_at"`
	LastUsed  time.Time `json:"last_used,omitempty"`
}

// FavoritesStorage manages favorite URLs
type FavoritesStorage struct {
	mu       sync.RWMutex
	filePath string
}

// NewFavoritesStorage creates a new favorites storage
func NewFavoritesStorage(dataDir string) (*FavoritesStorage, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	return &FavoritesStorage{
		filePath: filepath.Join(dataDir, "favorites.json"),
	}, nil
}

// Add adds a new favorite
func (s *FavoritesStorage) Add(name, url string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	favorites, err := s.loadUnsafe()
	if err != nil {
		favorites = make(map[string]*Favorite)
	}

	if _, exists := favorites[name]; exists {
		return fmt.Errorf("favorite '%s' already exists", name)
	}

	favorites[name] = &Favorite{
		Name:      name,
		URL:       url,
		CreatedAt: time.Now(),
	}

	return s.saveUnsafe(favorites)
}

// Get retrieves a favorite by name
func (s *FavoritesStorage) Get(name string) (*Favorite, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	favorites, err := s.loadUnsafe()
	if err != nil {
		return nil, err
	}

	fav, exists := favorites[name]
	if !exists {
		return nil, fmt.Errorf("favorite '%s' not found", name)
	}

	return fav, nil
}

// Remove removes a favorite
func (s *FavoritesStorage) Remove(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	favorites, err := s.loadUnsafe()
	if err != nil {
		return err
	}

	if _, exists := favorites[name]; !exists {
		return fmt.Errorf("favorite '%s' not found", name)
	}

	delete(favorites, name)
	return s.saveUnsafe(favorites)
}

// List returns all favorites
func (s *FavoritesStorage) List() ([]*Favorite, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	favorites, err := s.loadUnsafe()
	if err != nil {
		if os.IsNotExist(err) {
			return []*Favorite{}, nil
		}
		return nil, err
	}

	result := make([]*Favorite, 0, len(favorites))
	for _, fav := range favorites {
		result = append(result, fav)
	}

	return result, nil
}

// UpdateLastUsed updates the last used timestamp
func (s *FavoritesStorage) UpdateLastUsed(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	favorites, err := s.loadUnsafe()
	if err != nil {
		return err
	}

	fav, exists := favorites[name]
	if !exists {
		return fmt.Errorf("favorite '%s' not found", name)
	}

	fav.LastUsed = time.Now()
	return s.saveUnsafe(favorites)
}

// loadUnsafe loads favorites from file (no locking)
func (s *FavoritesStorage) loadUnsafe() (map[string]*Favorite, error) {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return nil, err
	}

	var favorites map[string]*Favorite
	if err := json.Unmarshal(data, &favorites); err != nil {
		return nil, fmt.Errorf("failed to parse favorites: %w", err)
	}

	return favorites, nil
}

// saveUnsafe saves favorites to file (no locking)
func (s *FavoritesStorage) saveUnsafe(favorites map[string]*Favorite) error {
	data, err := json.MarshalIndent(favorites, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal favorites: %w", err)
	}

	if err := os.WriteFile(s.filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write favorites: %w", err)
	}

	return nil
}
