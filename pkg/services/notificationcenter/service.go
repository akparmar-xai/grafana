package notificationcenter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/grafana/grafana/pkg/infra/kvstore"
	"github.com/grafana/grafana/pkg/util"
)

const (
	// namespace is the kv_store namespace for in-app notifications.
	// Rows live in the existing kv_store table, so this feature does not add a schema migration.
	namespace = "notification-center"
)

var (
	// ErrNotFound is returned when the id is not in the caller's own notifications.
	ErrNotFound = errors.New("notification not found")
	// ErrInvalidUser is returned when the caller has no user id.
	ErrInvalidUser = errors.New("invalid user")
)

type storedNotifications struct {
	Items []Notification `json:"items"`
}

// Service stores per-user notifications in the shared SQL kv store.
// Every method takes the caller's org and user ids. There is no way to address another user.
type Service struct {
	kv    kvstore.KVStore
	mu    sync.Mutex
	now   func() time.Time
	newID func() string
}

func ProvideService(kv kvstore.KVStore) *Service {
	return &Service{
		kv:    kv,
		now:   time.Now,
		newID: util.GenerateShortUID,
	}
}

// List returns the caller's notifications, newest first.
func (s *Service) List(ctx context.Context, orgID, userID int64) ([]Notification, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	items, err := s.load(ctx, orgID, userID)
	if err != nil {
		return nil, err
	}
	sortNewestFirst(items)
	return items, nil
}

// UnreadCount counts notifications the caller has not marked read.
func (s *Service) UnreadCount(ctx context.Context, orgID, userID int64) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	items, err := s.load(ctx, orgID, userID)
	if err != nil {
		return 0, err
	}
	return countUnread(items), nil
}

// MarkRead marks one of the caller's notifications read.
// An id that belongs to someone else is not found, because lookup never leaves this user's row.
func (s *Service) MarkRead(ctx context.Context, orgID, userID int64, id string) error {
	if id == "" {
		return fmt.Errorf("notification id is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	items, err := s.load(ctx, orgID, userID)
	if err != nil {
		return err
	}
	found := false
	for i := range items {
		if items[i].ID == id {
			items[i].Read = true
			found = true
			break
		}
	}
	if !found {
		return ErrNotFound
	}
	return s.save(ctx, orgID, userID, items)
}

// MarkAllRead marks every notification belonging to the caller as read.
func (s *Service) MarkAllRead(ctx context.Context, orgID, userID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	items, err := s.load(ctx, orgID, userID)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		return nil
	}
	for i := range items {
		items[i].Read = true
	}
	return s.save(ctx, orgID, userID, items)
}

// insert stores a notification for tests and for future producers.
// It is not exposed over HTTP, and production startup does not seed rows.
func (s *Service) insert(ctx context.Context, orgID, userID int64, n Notification) (Notification, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := userKey(userID); err != nil {
		return Notification{}, err
	}
	if n.ID == "" {
		n.ID = s.newID()
	}
	if n.Created.IsZero() {
		n.Created = s.now().UTC()
	}

	items, err := s.load(ctx, orgID, userID)
	if err != nil {
		return Notification{}, err
	}
	items = append(items, n)
	if err := s.save(ctx, orgID, userID, items); err != nil {
		return Notification{}, err
	}
	return n, nil
}

func (s *Service) load(ctx context.Context, orgID, userID int64) ([]Notification, error) {
	key, err := userKey(userID)
	if err != nil {
		return nil, err
	}
	raw, found, err := s.kv.Get(ctx, orgID, namespace, key)
	if err != nil {
		return nil, fmt.Errorf("read notifications: %w", err)
	}
	if !found || raw == "" {
		return []Notification{}, nil
	}
	var stored storedNotifications
	if err := json.Unmarshal([]byte(raw), &stored); err != nil {
		return nil, fmt.Errorf("decode notifications: %w", err)
	}
	if stored.Items == nil {
		return []Notification{}, nil
	}
	return stored.Items, nil
}

func (s *Service) save(ctx context.Context, orgID, userID int64, items []Notification) error {
	key, err := userKey(userID)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(storedNotifications{Items: items})
	if err != nil {
		return fmt.Errorf("encode notifications: %w", err)
	}
	if err := s.kv.Set(ctx, orgID, namespace, key, string(raw)); err != nil {
		return fmt.Errorf("write notifications: %w", err)
	}
	return nil
}

func userKey(userID int64) (string, error) {
	if userID <= 0 {
		return "", ErrInvalidUser
	}
	return strconv.FormatInt(userID, 10), nil
}

func countUnread(items []Notification) int {
	unread := 0
	for _, item := range items {
		if !item.Read {
			unread++
		}
	}
	return unread
}

func sortNewestFirst(items []Notification) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].Created.Equal(items[j].Created) {
			return items[i].ID > items[j].ID
		}
		return items[i].Created.After(items[j].Created)
	})
}
