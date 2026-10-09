// Package customer is the customer domain: the entity, its invariants and the
// operations on it. It must not import transport or generated contract code;
// internal/customer/customerrpc adapts it to the API.
package customer

import (
	"cmp"
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

// Change lists the fields an update modifies; nil fields keep their value.
type Change struct {
	Name  *string
	Email *string
}

// Update applies change to an existing customer. Read, merge and write happen
// under one lock so concurrent partial updates cannot lose each other's
// fields.
func (s *Store) Update(ctx context.Context, id uuid.UUID, change Change) (Customer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := s.byID[id]
	if !ok {
		return Customer{}, fmt.Errorf("%w: %s", ErrNotFound, id)
	}
	name, email := current.Name, current.Email
	if change.Name != nil {
		name = *change.Name
	}
	if change.Email != nil {
		email = *change.Email
	}
	updated, err := build(id, name, email)
	if err != nil {
		return Customer{}, err
	}
	s.byID[id] = updated
	return updated, nil
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

// Filter selects customers. The zero value matches every customer; set
// fields are combined with AND.
type Filter struct {
	NameContains string // case-insensitive substring
	Email        string // case-insensitive exact match
}

func (f Filter) matches(c Customer) bool {
	if f.NameContains != "" && !strings.Contains(strings.ToLower(c.Name), strings.ToLower(f.NameContains)) {
		return false
	}
	if f.Email != "" && !strings.EqualFold(c.Email, f.Email) {
		return false
	}
	return true
}

type SortKey int

const (
	SortByCreation SortKey = iota
	SortByName
	SortByEmail
)

// Order sorts customers. The zero value is creation order, ascending. Ties
// on the sort key are broken by ID, which keeps the order total.
type Order struct {
	Key        SortKey
	Descending bool
}

// Cursor is a position in an ordered listing: the sort key value and ID of
// the last customer already returned. The zero value is the start.
type Cursor struct {
	Key string
	ID  uuid.UUID
}

func (o Order) cursor(c Customer) Cursor {
	switch o.Key {
	case SortByName:
		return Cursor{Key: strings.ToLower(c.Name), ID: c.ID}
	case SortByEmail:
		return Cursor{Key: strings.ToLower(c.Email), ID: c.ID}
	default:
		return Cursor{ID: c.ID}
	}
}

func (o Order) compare(a, b Cursor) int {
	r := cmp.Or(strings.Compare(a.Key, b.Key), a.ID.Compare(b.ID))
	if o.Descending {
		return -r
	}
	return r
}

// List returns up to limit customers matching filter, sorted by order and
// positioned after the cursor. next is the cursor of the following page, or
// the zero Cursor when no matching customers remain. UUIDv7 IDs make ID
// order creation order, so pages stay stable while customers are added.
func (s *Store) List(ctx context.Context, filter Filter, order Order, after Cursor, limit int) (page []Customer, next Cursor, err error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, c := range s.byID {
		if !filter.matches(c) {
			continue
		}
		if after != (Cursor{}) && order.compare(order.cursor(c), after) <= 0 {
			continue
		}
		page = append(page, c)
	}
	slices.SortFunc(page, func(a, b Customer) int { return order.compare(order.cursor(a), order.cursor(b)) })
	if len(page) > limit {
		page = page[:limit]
		return page, order.cursor(page[limit-1]), nil
	}
	return page, Cursor{}, nil
}

// Exists reports whether a customer exists. Other contexts use it to check
// references without depending on Customer itself.
func (s *Store) Exists(ctx context.Context, id uuid.UUID) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.byID[id]
	return ok, nil
}
