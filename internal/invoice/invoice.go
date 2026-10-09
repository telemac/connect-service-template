// Package invoice is the invoice domain: the entity, its lifecycle and the
// operations on it. It references customers by ID only and checks them
// through the narrow Customers interface, so it does not import the customer
// package; cmd/server wires the two together.
package invoice

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"
	"uuid"
)

var (
	ErrNotFound = errors.New("invoice not found")
	ErrInvalid  = errors.New("invalid invoice")
	// ErrUnknownCustomer means the referenced customer does not exist.
	ErrUnknownCustomer = errors.New("unknown customer")
	// ErrTransition means the requested status change is not allowed from the
	// invoice's current status.
	ErrTransition = errors.New("invalid status transition")
)

// Status is the invoice lifecycle. The zero value is not a valid status.
type Status int

const (
	Draft Status = iota + 1
	Issued
	Paid
	Void
)

func (s Status) String() string {
	switch s {
	case Draft:
		return "draft"
	case Issued:
		return "issued"
	case Paid:
		return "paid"
	case Void:
		return "void"
	default:
		return fmt.Sprintf("status(%d)", int(s))
	}
}

// canBecome encodes the lifecycle: DRAFT -> ISSUED -> PAID, and DRAFT or
// ISSUED -> VOID. PAID and VOID are final.
func (s Status) canBecome(to Status) bool {
	switch to {
	case Issued:
		return s == Draft
	case Paid:
		return s == Issued
	case Void:
		return s == Draft || s == Issued
	default:
		return false
	}
}

type Invoice struct {
	ID          uuid.UUID
	CustomerID  uuid.UUID
	AmountCents int64
	Currency    string
	Status      Status
	CreateTime  time.Time
	UpdateTime  time.Time
}

// Customers is the view of the customer context this package needs.
type Customers interface {
	Exists(ctx context.Context, id uuid.UUID) (bool, error)
}

// Store keeps invoices in memory. Persistence is out of scope for v1; a
// database-backed store replaces it without changing callers.
type Store struct {
	customers Customers
	mu        sync.RWMutex
	byID      map[uuid.UUID]Invoice
}

func NewStore(customers Customers) *Store {
	return &Store{customers: customers, byID: make(map[uuid.UUID]Invoice)}
}

// Create adds a DRAFT invoice. The customer check is not atomic with the
// insert; a database would enforce the reference with a foreign key.
func (s *Store) Create(ctx context.Context, customerID uuid.UUID, amountCents int64, currency string) (Invoice, error) {
	if amountCents <= 0 {
		return Invoice{}, fmt.Errorf("%w: amount must be positive", ErrInvalid)
	}
	if !isCurrencyCode(currency) {
		return Invoice{}, fmt.Errorf("%w: currency %q is not an ISO 4217 code", ErrInvalid, currency)
	}
	ok, err := s.customers.Exists(ctx, customerID)
	if err != nil {
		return Invoice{}, fmt.Errorf("checking customer %s: %w", customerID, err)
	}
	if !ok {
		return Invoice{}, fmt.Errorf("%w: %s", ErrUnknownCustomer, customerID)
	}

	now := time.Now()
	inv := Invoice{
		ID:          uuid.NewV7(),
		CustomerID:  customerID,
		AmountCents: amountCents,
		Currency:    currency,
		Status:      Draft,
		CreateTime:  now,
		UpdateTime:  now,
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byID[inv.ID] = inv
	return inv, nil
}

func isCurrencyCode(s string) bool {
	if len(s) != 3 {
		return false
	}
	for _, r := range s {
		if r < 'A' || r > 'Z' {
			return false
		}
	}
	return true
}

func (s *Store) Get(ctx context.Context, id uuid.UUID) (Invoice, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	inv, ok := s.byID[id]
	if !ok {
		return Invoice{}, fmt.Errorf("%w: %s", ErrNotFound, id)
	}
	return inv, nil
}

func (s *Store) Issue(ctx context.Context, id uuid.UUID) (Invoice, error) {
	return s.transition(id, Issued)
}

func (s *Store) Pay(ctx context.Context, id uuid.UUID) (Invoice, error) {
	return s.transition(id, Paid)
}

func (s *Store) Void(ctx context.Context, id uuid.UUID) (Invoice, error) {
	return s.transition(id, Void)
}

func (s *Store) transition(id uuid.UUID, to Status) (Invoice, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	inv, ok := s.byID[id]
	if !ok {
		return Invoice{}, fmt.Errorf("%w: %s", ErrNotFound, id)
	}
	if !inv.Status.canBecome(to) {
		return Invoice{}, fmt.Errorf("%w: %s to %s", ErrTransition, inv.Status, to)
	}
	inv.Status = to
	inv.UpdateTime = time.Now()
	s.byID[id] = inv
	return inv, nil
}

// Filter selects invoices. The zero value matches every invoice; set fields
// are combined with AND.
type Filter struct {
	CustomerID uuid.UUID
	Status     Status
}

func (f Filter) matches(inv Invoice) bool {
	if f.CustomerID != uuid.Nil() && inv.CustomerID != f.CustomerID {
		return false
	}
	if f.Status != 0 && inv.Status != f.Status {
		return false
	}
	return true
}

// List returns up to limit invoices matching filter with an ID greater than
// after, in ID order, which UUIDv7 makes creation order. next is the cursor
// of the following page, or uuid.Nil() when no matching invoices remain.
func (s *Store) List(ctx context.Context, filter Filter, after uuid.UUID, limit int) (page []Invoice, next uuid.UUID, err error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, inv := range s.byID {
		if inv.ID.Compare(after) > 0 && filter.matches(inv) {
			page = append(page, inv)
		}
	}
	slices.SortFunc(page, func(a, b Invoice) int { return a.ID.Compare(b.ID) })
	if len(page) > limit {
		page = page[:limit]
		return page, page[limit-1].ID, nil
	}
	return page, uuid.Nil(), nil
}
