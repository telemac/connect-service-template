package invoice_test

import (
	"context"
	"errors"
	"testing"
	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/telemac/connectrpc-v2-poc/internal/invoice"
)

// customers is a fake invoice.Customers: every ID in the set exists.
type customers map[uuid.UUID]bool

func (c customers) Exists(ctx context.Context, id uuid.UUID) (bool, error) {
	return c[id], nil
}

func newStore(t *testing.T) (*invoice.Store, uuid.UUID) {
	t.Helper()
	known := uuid.NewV7()
	return invoice.NewStore(customers{known: true}), known
}

func TestCreateStartsAsDraft(t *testing.T) {
	assert := assert.New(t)
	store, customerID := newStore(t)

	inv, err := store.Create(t.Context(), customerID, 1250, "EUR")
	assert.NoError(err)
	assert.Equal(invoice.Draft, inv.Status)
	assert.False(inv.CreateTime.IsZero())
	assert.Equal(inv.CreateTime, inv.UpdateTime)
}

func TestCreateRejects(t *testing.T) {
	store, customerID := newStore(t)
	tests := []struct {
		name       string
		customerID uuid.UUID
		amount     int64
		currency   string
		want       error
	}{
		{"zero amount", customerID, 0, "EUR", invoice.ErrInvalid},
		{"lowercase currency", customerID, 100, "eur", invoice.ErrInvalid},
		{"unknown customer", uuid.NewV7(), 100, "EUR", invoice.ErrUnknownCustomer},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert := assert.New(t)
			_, err := store.Create(t.Context(), tt.customerID, tt.amount, tt.currency)
			assert.ErrorIs(err, tt.want)
		})
	}
}

func TestCreatePropagatesCustomerLookupErrors(t *testing.T) {
	assert := assert.New(t)
	boom := errors.New("customer store down")
	store := invoice.NewStore(failingCustomers{boom})

	_, err := store.Create(t.Context(), uuid.NewV7(), 100, "EUR")
	assert.ErrorIs(err, boom)
}

type failingCustomers struct{ err error }

func (f failingCustomers) Exists(ctx context.Context, id uuid.UUID) (bool, error) {
	return false, f.err
}

func TestLifecycle(t *testing.T) {
	type step func(*invoice.Store, context.Context, uuid.UUID) (invoice.Invoice, error)
	issue, pay, void := (*invoice.Store).Issue, (*invoice.Store).Pay, (*invoice.Store).Void

	tests := []struct {
		name  string
		steps []step
		want  invoice.Status
		err   error
	}{
		{"issue then pay", []step{issue, pay}, invoice.Paid, nil},
		{"void a draft", []step{void}, invoice.Void, nil},
		{"void an issued invoice", []step{issue, void}, invoice.Void, nil},
		{"pay a draft", []step{pay}, invoice.Draft, invoice.ErrTransition},
		{"issue twice", []step{issue, issue}, invoice.Issued, invoice.ErrTransition},
		{"void a paid invoice", []step{issue, pay, void}, invoice.Paid, invoice.ErrTransition},
		{"pay a void invoice", []step{void, pay}, invoice.Void, invoice.ErrTransition},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert := assert.New(t)
			store, customerID := newStore(t)
			inv, err := store.Create(t.Context(), customerID, 100, "EUR")
			require.NoError(t, err)

			for _, s := range tt.steps {
				_, err = s(store, t.Context(), inv.ID)
			}
			if tt.err != nil {
				assert.ErrorIs(err, tt.err)
			} else {
				assert.NoError(err)
			}
			got, err := store.Get(t.Context(), inv.ID)
			assert.NoError(err)
			assert.Equal(tt.want, got.Status, "a rejected transition must not change the status")
		})
	}
}

func TestTransitionOnUnknownInvoice(t *testing.T) {
	assert := assert.New(t)
	store, _ := newStore(t)
	_, err := store.Issue(t.Context(), uuid.NewV7())
	assert.ErrorIs(err, invoice.ErrNotFound)
}

func TestListFiltersAndPages(t *testing.T) {
	assert := assert.New(t)
	ada, grace := uuid.NewV7(), uuid.NewV7()
	store := invoice.NewStore(customers{ada: true, grace: true})
	var adaInvoices []uuid.UUID
	for _, c := range []uuid.UUID{ada, grace, ada, ada} {
		inv, err := store.Create(t.Context(), c, 100, "EUR")
		require.NoError(t, err)
		if c == ada {
			adaInvoices = append(adaInvoices, inv.ID)
		}
	}
	_, err := store.Issue(t.Context(), adaInvoices[1])
	require.NoError(t, err)

	var got []uuid.UUID
	after := uuid.Nil()
	for range 10 {
		page, next, err := store.List(t.Context(), invoice.Filter{CustomerID: ada}, after, 2)
		assert.NoError(err)
		for _, inv := range page {
			got = append(got, inv.ID)
		}
		if next == uuid.Nil() {
			break
		}
		after = next
	}
	assert.Equal(adaInvoices, got)

	issued, _, err := store.List(t.Context(), invoice.Filter{CustomerID: ada, Status: invoice.Issued}, uuid.Nil(), 10)
	assert.NoError(err)
	if assert.Len(issued, 1) {
		assert.Equal(adaInvoices[1], issued[0].ID)
	}
}
