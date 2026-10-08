package customerrpc_test

import (
	"errors"
	"io"
	"log/slog"
	"testing"
	"uuid"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connectinprocess"
	"connectrpc.com/validate"
	"github.com/stretchr/testify/assert"

	customerv1 "github.com/telemac/connectrpc-v2-poc/gen/go/customer/v1"
	"github.com/telemac/connectrpc-v2-poc/gen/go/customer/v1/customerv1connect"
	"github.com/telemac/connectrpc-v2-poc/internal/customer"
	"github.com/telemac/connectrpc-v2-poc/internal/customer/customerrpc"
)

func newClient(t *testing.T) customerv1connect.CustomerServiceClient {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := connect.NewServer(validate.NewServerInterceptor())
	customerv1connect.RegisterCustomerServiceHandler(server, customerrpc.NewHandler(customer.NewStore(), logger))
	return customerv1connect.NewCustomerServiceClient(connect.NewClient(connectinprocess.New(server)))
}

func codeOf(err error) connect.Code {
	if connectErr, ok := errors.AsType[*connect.Error](err); ok {
		return connectErr.Code()
	}
	return connect.CodeUnknown
}

func TestCreatedCustomerCanBeFetched(t *testing.T) {
	assert := assert.New(t)
	client := newClient(t)

	created, err := client.CreateCustomer(t.Context(), &customerv1.CreateCustomerRequest{Customer: &customerv1.Customer{Name: "Ada", Email: "ada@example.com"}})
	assert.NoError(err)

	got, err := client.GetCustomer(t.Context(), &customerv1.GetCustomerRequest{Id: created.GetCustomer().GetId()})
	assert.NoError(err)
	assert.Equal("Ada", got.GetCustomer().GetName())
	assert.Equal("ada@example.com", got.GetCustomer().GetEmail())
}

func TestUpdatedCustomerIsReturnedAndStored(t *testing.T) {
	assert := assert.New(t)
	client := newClient(t)
	created, err := client.CreateCustomer(t.Context(), &customerv1.CreateCustomerRequest{Customer: &customerv1.Customer{Name: "Ada", Email: "ada@example.com"}})
	assert.NoError(err)
	id := created.GetCustomer().GetId()

	updated, err := client.UpdateCustomer(t.Context(), &customerv1.UpdateCustomerRequest{
		Id:       id,
		Customer: &customerv1.Customer{Id: uuid.NewV7().String(), Name: "Ada Lovelace", Email: "ada@lovelace.org"},
	})
	assert.NoError(err)
	assert.Equal(id, updated.GetCustomer().GetId(), "customer.id in the body must be ignored")

	got, err := client.GetCustomer(t.Context(), &customerv1.GetCustomerRequest{Id: id})
	assert.NoError(err)
	assert.Equal("Ada Lovelace", got.GetCustomer().GetName())
	assert.Equal("ada@lovelace.org", got.GetCustomer().GetEmail())
}

func TestErrorsMapToConnectCodes(t *testing.T) {
	tests := []struct {
		name string
		call func(customerv1connect.CustomerServiceClient) error
		want connect.Code
	}{
		{
			name: "contract validation rejects a malformed email",
			call: func(c customerv1connect.CustomerServiceClient) error {
				_, err := c.CreateCustomer(t.Context(), &customerv1.CreateCustomerRequest{Customer: &customerv1.Customer{Name: "Ada", Email: "not-an-email"}})
				return err
			},
			want: connect.CodeInvalidArgument,
		},
		{
			name: "contract validation requires the customer",
			call: func(c customerv1connect.CustomerServiceClient) error {
				_, err := c.CreateCustomer(t.Context(), &customerv1.CreateCustomerRequest{})
				return err
			},
			want: connect.CodeInvalidArgument,
		},
		{
			name: "update requires the customer",
			call: func(c customerv1connect.CustomerServiceClient) error {
				_, err := c.UpdateCustomer(t.Context(), &customerv1.UpdateCustomerRequest{Id: uuid.NewV7().String()})
				return err
			},
			want: connect.CodeInvalidArgument,
		},
		{
			name: "updating an unknown customer is not found",
			call: func(c customerv1connect.CustomerServiceClient) error {
				_, err := c.UpdateCustomer(t.Context(), &customerv1.UpdateCustomerRequest{
					Id:       uuid.NewV7().String(),
					Customer: &customerv1.Customer{Name: "Ada", Email: "ada@example.com"},
				})
				return err
			},
			want: connect.CodeNotFound,
		},
		{
			name: "unknown customer is not found",
			call: func(c customerv1connect.CustomerServiceClient) error {
				_, err := c.GetCustomer(t.Context(), &customerv1.GetCustomerRequest{Id: uuid.NewV7().String()})
				return err
			},
			want: connect.CodeNotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert := assert.New(t)
			assert.Equal(tt.want, codeOf(tt.call(newClient(t))))
		})
	}
}
