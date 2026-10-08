package customer_test

import (
	"testing"
	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/telemac/connectrpc-v2-poc/internal/customer"
)

func TestNewRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name, customerName, email string
	}{
		{"blank name", "  ", "ada@example.com"},
		{"malformed email", "Ada", "ada"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert := assert.New(t)
			_, err := customer.New(tt.customerName, tt.email)
			assert.ErrorIs(err, customer.ErrInvalid)
		})
	}
}

func TestStoreReturnsCreatedCustomer(t *testing.T) {
	assert := assert.New(t)
	store := customer.NewStore()

	created, err := store.Create(t.Context(), "Ada", "ada@example.com")
	assert.NoError(err)

	got, err := store.Get(t.Context(), created.ID)
	assert.NoError(err)
	assert.Equal(created, got)
}

func TestStoreReportsUnknownCustomer(t *testing.T) {
	assert := assert.New(t)
	_, err := customer.NewStore().Get(t.Context(), uuid.NewV7())
	assert.ErrorIs(err, customer.ErrNotFound)
}

func TestStoreUpdateReplacesFields(t *testing.T) {
	assert := assert.New(t)
	store := customer.NewStore()
	created, err := store.Create(t.Context(), "Ada", "ada@example.com")
	assert.NoError(err)

	updated, err := store.Update(t.Context(), created.ID, "Ada Lovelace", "ada@lovelace.org")
	assert.NoError(err)
	assert.Equal(customer.Customer{ID: created.ID, Name: "Ada Lovelace", Email: "ada@lovelace.org"}, updated)

	got, err := store.Get(t.Context(), created.ID)
	assert.NoError(err)
	assert.Equal(updated, got)
}

func TestStoreUpdateRejectsUnknownOrInvalid(t *testing.T) {
	assert := assert.New(t)
	store := customer.NewStore()
	created, err := store.Create(t.Context(), "Ada", "ada@example.com")
	assert.NoError(err)

	_, err = store.Update(t.Context(), uuid.NewV7(), "Ada", "ada@example.com")
	assert.ErrorIs(err, customer.ErrNotFound)

	_, err = store.Update(t.Context(), created.ID, " ", "ada@example.com")
	assert.ErrorIs(err, customer.ErrInvalid)
}

func TestStoreDeleteRemovesCustomer(t *testing.T) {
	assert := assert.New(t)
	store := customer.NewStore()
	created, err := store.Create(t.Context(), "Ada", "ada@example.com")
	assert.NoError(err)

	assert.NoError(store.Delete(t.Context(), created.ID))
	_, err = store.Get(t.Context(), created.ID)
	assert.ErrorIs(err, customer.ErrNotFound)
	assert.ErrorIs(store.Delete(t.Context(), created.ID), customer.ErrNotFound)
}

func TestStoreListPagesInCreationOrder(t *testing.T) {
	assert := assert.New(t)
	store := customer.NewStore()
	var want []customer.Customer
	for _, name := range []string{"Ada", "Grace", "Linus"} {
		c, err := store.Create(t.Context(), name, "x@example.com")
		assert.NoError(err)
		want = append(want, c)
	}

	first, more, err := store.List(t.Context(), customer.Filter{}, uuid.Nil(), 2)
	assert.NoError(err)
	assert.True(more)
	assert.Equal(want[:2], first)

	second, more, err := store.List(t.Context(), customer.Filter{}, first[1].ID, 2)
	assert.NoError(err)
	assert.False(more)
	assert.Equal(want[2:], second)
}

func TestStoreListFilters(t *testing.T) {
	store := customer.NewStore()
	for _, c := range []struct{ name, email string }{
		{"Ada Lovelace", "ada@example.com"},
		{"Grace Hopper", "grace@example.com"},
		{"Ada Byron", "byron@example.org"},
	} {
		_, err := store.Create(t.Context(), c.name, c.email)
		require.NoError(t, err)
	}

	tests := []struct {
		name   string
		filter customer.Filter
		want   []string
	}{
		{"zero filter matches all", customer.Filter{}, []string{"Ada Lovelace", "Grace Hopper", "Ada Byron"}},
		{"name substring ignores case", customer.Filter{NameContains: "ADA"}, []string{"Ada Lovelace", "Ada Byron"}},
		{"email exact ignores case", customer.Filter{Email: "Grace@Example.com"}, []string{"Grace Hopper"}},
		{"fields combine with AND", customer.Filter{NameContains: "ada", Email: "byron@example.org"}, []string{"Ada Byron"}},
		{"no match", customer.Filter{NameContains: "linus"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert := assert.New(t)
			page, more, err := store.List(t.Context(), tt.filter, uuid.Nil(), 10)
			assert.NoError(err)
			assert.False(more)
			var got []string
			for _, c := range page {
				got = append(got, c.Name)
			}
			assert.Equal(tt.want, got)
		})
	}
}
