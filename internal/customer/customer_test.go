package customer_test

import (
	"testing"
	"uuid"

	"github.com/stretchr/testify/assert"

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
