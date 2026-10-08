// Package customer is the customer domain: the entity, its invariants and the
// operations on it. It must not import transport or generated contract code;
// internal/customer/customerrpc adapts it to the API.
package customer

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"uuid"
)

var (
	ErrNotFound = errors.New("customer not found")
	ErrInvalid  = errors.New("invalid customer")
)

type Customer struct {
	ID    uuid.UUID
	Name  string
	Email string
}

// New returns a customer with a fresh ID. The contract already validates
// requests; these checks keep the invariants true for any other caller.
func New(name, email string) (Customer, error) {
	return build(uuid.NewV7(), name, email)
}

func build(id uuid.UUID, name, email string) (Customer, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Customer{}, fmt.Errorf("%w: name is required", ErrInvalid)
	}
	if !strings.Contains(email, "@") {
		return Customer{}, fmt.Errorf("%w: email %q is malformed", ErrInvalid, email)
	}
	return Customer{ID: id, Name: name, Email: email}, nil
}

// Store keeps customers in memory. Persistence is out of scope for v1; a
// database-backed store replaces it without changing callers.
type Store struct {
	mu   sync.RWMutex
	byID map[uuid.UUID]Customer
}

func NewStore() *Store {
	return &Store{byID: make(map[uuid.UUID]Customer)}
}

func (s *Store) Create(ctx context.Context, name, email string) (Customer, error) {
	c, err := New(name, email)
	if err != nil {
		return Customer{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byID[c.ID] = c
	return c, nil
}

func (s *Store) Get(ctx context.Context, id uuid.UUID) (Customer, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.byID[id]
	if !ok {
		return Customer{}, fmt.Errorf("%w: %s", ErrNotFound, id)
	}
	return c, nil
}

// Update replaces the mutable fields of an existing customer.
func (s *Store) Update(ctx context.Context, id uuid.UUID, name, email string) (Customer, error) {
	c, err := build(id, name, email)
	if err != nil {
		return Customer{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.byID[id]; !ok {
		return Customer{}, fmt.Errorf("%w: %s", ErrNotFound, id)
	}
	s.byID[id] = c
	return c, nil
}

func (s *Store) Delete(ctx context.Context, id uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.byID[id]; !ok {
		return fmt.Errorf("%w: %s", ErrNotFound, id)
	}
	delete(s.byID, id)
	return nil
}

// List returns up to limit customers with an ID greater than after, in ID
// order. UUIDv7 IDs make that creation order, so pages stay stable while
// customers are added. more reports whether customers remain after the page.
func (s *Store) List(ctx context.Context, after uuid.UUID, limit int) (page []Customer, more bool, err error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, c := range s.byID {
		if c.ID.Compare(after) > 0 {
			page = append(page, c)
		}
	}
	slices.SortFunc(page, func(a, b Customer) int { return a.ID.Compare(b.ID) })
	if len(page) > limit {
		return page[:limit], true, nil
	}
	return page, false, nil
}
