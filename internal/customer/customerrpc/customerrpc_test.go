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
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

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

func TestUpdateCustomerAppliesMask(t *testing.T) {
	tests := []struct {
		name      string
		mask      []string // nil means omitted
		body      *customerv1.Customer
		wantName  string
		wantEmail string
	}{
		{"mask limits the change", []string{"email"}, &customerv1.Customer{Name: "Ignored", Email: "ada@lovelace.org"}, "Ada", "ada@lovelace.org"},
		{"omitted mask uses populated fields", nil, &customerv1.Customer{Name: "Ada Lovelace"}, "Ada Lovelace", "ada@example.com"},
		{"wildcard replaces everything", []string{"*"}, &customerv1.Customer{Name: "Grace", Email: "grace@example.com"}, "Grace", "grace@example.com"},
		{"output only id is ignored", []string{"id", "name"}, &customerv1.Customer{Name: "Ada Lovelace"}, "Ada Lovelace", "ada@example.com"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert := assert.New(t)
			client := newClient(t)
			created, err := client.CreateCustomer(t.Context(), &customerv1.CreateCustomerRequest{Customer: &customerv1.Customer{Name: "Ada", Email: "ada@example.com"}})
			require.NoError(t, err)
			id := created.GetCustomer().GetId()

			req := &customerv1.UpdateCustomerRequest{Id: id, Customer: tt.body}
			if tt.mask != nil {
				req.UpdateMask = &fieldmaskpb.FieldMask{Paths: tt.mask}
			}
			updated, err := client.UpdateCustomer(t.Context(), req)
			assert.NoError(err)
			assert.Equal(id, updated.GetCustomer().GetId())

			got, err := client.GetCustomer(t.Context(), &customerv1.GetCustomerRequest{Id: id})
			assert.NoError(err)
			assert.Equal(tt.wantName, got.GetCustomer().GetName())
			assert.Equal(tt.wantEmail, got.GetCustomer().GetEmail())
		})
	}
}

func TestDeletedCustomerIsGone(t *testing.T) {
	assert := assert.New(t)
	client := newClient(t)
	created, err := client.CreateCustomer(t.Context(), &customerv1.CreateCustomerRequest{Customer: &customerv1.Customer{Name: "Ada", Email: "ada@example.com"}})
	assert.NoError(err)
	id := created.GetCustomer().GetId()

	_, err = client.DeleteCustomer(t.Context(), &customerv1.DeleteCustomerRequest{Id: id})
	assert.NoError(err)

	_, err = client.GetCustomer(t.Context(), &customerv1.GetCustomerRequest{Id: id})
	assert.Equal(connect.CodeNotFound, codeOf(err))
}

func TestListCustomersFollowsPageTokens(t *testing.T) {
	assert := assert.New(t)
	client := newClient(t)
	var want []string
	for _, name := range []string{"Ada", "Grace", "Linus"} {
		resp, err := client.CreateCustomer(t.Context(), &customerv1.CreateCustomerRequest{Customer: &customerv1.Customer{Name: name, Email: "x@example.com"}})
		assert.NoError(err)
		want = append(want, resp.GetCustomer().GetId())
	}

	var got []string
	token := ""
	for pages := 0; pages < 10; pages++ {
		resp, err := client.ListCustomers(t.Context(), &customerv1.ListCustomersRequest{PageSize: 2, PageToken: token})
		assert.NoError(err)
		for _, c := range resp.GetCustomers() {
			got = append(got, c.GetId())
		}
		token = resp.GetNextPageToken()
		if token == "" {
			break
		}
	}
	assert.Equal(want, got)
}

func TestListCustomersWithoutPageSizeUsesDefaultPage(t *testing.T) {
	assert := assert.New(t)
	client := newClient(t)
	for _, name := range []string{"Ada", "Grace", "Linus"} {
		_, err := client.CreateCustomer(t.Context(), &customerv1.CreateCustomerRequest{Customer: &customerv1.Customer{Name: name, Email: "x@example.com"}})
		assert.NoError(err)
	}

	resp, err := client.ListCustomers(t.Context(), &customerv1.ListCustomersRequest{})
	assert.NoError(err)
	assert.Len(resp.GetCustomers(), 3)
	assert.Empty(resp.GetNextPageToken())
}

func TestListCustomersAppliesFilters(t *testing.T) {
	assert := assert.New(t)
	client := newClient(t)
	for _, c := range []struct{ name, email string }{{"Ada", "ada@example.com"}, {"Grace", "grace@example.com"}} {
		_, err := client.CreateCustomer(t.Context(), &customerv1.CreateCustomerRequest{Customer: &customerv1.Customer{Name: c.name, Email: c.email}})
		assert.NoError(err)
	}

	resp, err := client.ListCustomers(t.Context(), &customerv1.ListCustomersRequest{NameContains: "gra", Email: "grace@example.com"})
	assert.NoError(err)
	if assert.Len(resp.GetCustomers(), 1) {
		assert.Equal("Grace", resp.GetCustomers()[0].GetName())
	}
}

func TestListCustomersSortsAcrossPages(t *testing.T) {
	assert := assert.New(t)
	client := newClient(t)
	for _, name := range []string{"Grace", "Ada", "Linus"} {
		_, err := client.CreateCustomer(t.Context(), &customerv1.CreateCustomerRequest{Customer: &customerv1.Customer{Name: name, Email: "x@example.com"}})
		assert.NoError(err)
	}

	var got []string
	token := ""
	for range 10 {
		resp, err := client.ListCustomers(t.Context(), &customerv1.ListCustomersRequest{
			PageSize:   1,
			PageToken:  token,
			OrderBy:    customerv1.CustomerOrder_CUSTOMER_ORDER_NAME,
			Descending: true,
		})
		assert.NoError(err)
		for _, c := range resp.GetCustomers() {
			got = append(got, c.GetName())
		}
		if token = resp.GetNextPageToken(); token == "" {
			break
		}
	}
	assert.Equal([]string{"Linus", "Grace", "Ada"}, got)
}

func TestListCustomersRejectsTokenReplayedWithOtherParameters(t *testing.T) {
	client := newClient(t)
	for _, name := range []string{"Ada", "Adele", "Grace"} {
		_, err := client.CreateCustomer(t.Context(), &customerv1.CreateCustomerRequest{Customer: &customerv1.Customer{Name: name, Email: "x@example.com"}})
		require.NoError(t, err)
	}
	issued := &customerv1.ListCustomersRequest{PageSize: 1, NameContains: "ad", OrderBy: customerv1.CustomerOrder_CUSTOMER_ORDER_NAME}
	first, err := client.ListCustomers(t.Context(), issued)
	require.NoError(t, err)
	token := first.GetNextPageToken()
	require.NotEmpty(t, token)

	tests := []struct {
		name     string
		req      *customerv1.ListCustomersRequest
		rejected bool
	}{
		{"same parameters are accepted", &customerv1.ListCustomersRequest{PageSize: 1, NameContains: "ad", OrderBy: customerv1.CustomerOrder_CUSTOMER_ORDER_NAME}, false},
		{"other order", &customerv1.ListCustomersRequest{PageSize: 1, NameContains: "ad"}, true},
		{"other direction", &customerv1.ListCustomersRequest{PageSize: 1, NameContains: "ad", OrderBy: customerv1.CustomerOrder_CUSTOMER_ORDER_NAME, Descending: true}, true},
		{"other name filter", &customerv1.ListCustomersRequest{PageSize: 1, NameContains: "gr", OrderBy: customerv1.CustomerOrder_CUSTOMER_ORDER_NAME}, true},
		{"added email filter", &customerv1.ListCustomersRequest{PageSize: 1, NameContains: "ad", Email: "x@example.com", OrderBy: customerv1.CustomerOrder_CUSTOMER_ORDER_NAME}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert := assert.New(t)
			tt.req.PageToken = token
			_, err := client.ListCustomers(t.Context(), tt.req)
			if tt.rejected {
				assert.Equal(connect.CodeInvalidArgument, codeOf(err))
			} else {
				assert.NoError(err)
			}
		})
	}
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
			name: "create requires the name",
			call: func(c customerv1connect.CustomerServiceClient) error {
				_, err := c.CreateCustomer(t.Context(), &customerv1.CreateCustomerRequest{Customer: &customerv1.Customer{Email: "ada@example.com"}})
				return err
			},
			want: connect.CodeInvalidArgument,
		},
		{
			name: "update rejects an unknown mask path",
			call: func(c customerv1connect.CustomerServiceClient) error {
				_, err := c.UpdateCustomer(t.Context(), &customerv1.UpdateCustomerRequest{
					Id:         uuid.NewV7().String(),
					Customer:   &customerv1.Customer{Name: "Ada"},
					UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"phone"}},
				})
				return err
			},
			want: connect.CodeInvalidArgument,
		},
		{
			name: "update requires fields named in the mask",
			call: func(c customerv1connect.CustomerServiceClient) error {
				_, err := c.UpdateCustomer(t.Context(), &customerv1.UpdateCustomerRequest{
					Id:         uuid.NewV7().String(),
					Customer:   &customerv1.Customer{Email: "ada@example.com"},
					UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"name"}},
				})
				return err
			},
			want: connect.CodeInvalidArgument,
		},
		{
			name: "wildcard update requires every field",
			call: func(c customerv1connect.CustomerServiceClient) error {
				_, err := c.UpdateCustomer(t.Context(), &customerv1.UpdateCustomerRequest{
					Id:         uuid.NewV7().String(),
					Customer:   &customerv1.Customer{Email: "ada@example.com"},
					UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"*"}},
				})
				return err
			},
			want: connect.CodeInvalidArgument,
		},
		{
			name: "update rejects a body id different from the target id",
			call: func(c customerv1connect.CustomerServiceClient) error {
				_, err := c.UpdateCustomer(t.Context(), &customerv1.UpdateCustomerRequest{
					Id:       uuid.NewV7().String(),
					Customer: &customerv1.Customer{Id: uuid.NewV7().String(), Name: "Ada", Email: "ada@example.com"},
				})
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
			name: "deleting an unknown customer is not found",
			call: func(c customerv1connect.CustomerServiceClient) error {
				_, err := c.DeleteCustomer(t.Context(), &customerv1.DeleteCustomerRequest{Id: uuid.NewV7().String()})
				return err
			},
			want: connect.CodeNotFound,
		},
		{
			name: "delete rejects a malformed id",
			call: func(c customerv1connect.CustomerServiceClient) error {
				_, err := c.DeleteCustomer(t.Context(), &customerv1.DeleteCustomerRequest{Id: "not-a-uuid"})
				return err
			},
			want: connect.CodeInvalidArgument,
		},
		{
			name: "list rejects a negative page size",
			call: func(c customerv1connect.CustomerServiceClient) error {
				_, err := c.ListCustomers(t.Context(), &customerv1.ListCustomersRequest{PageSize: -1})
				return err
			},
			want: connect.CodeInvalidArgument,
		},
		{
			name: "list rejects a malformed page token",
			call: func(c customerv1connect.CustomerServiceClient) error {
				_, err := c.ListCustomers(t.Context(), &customerv1.ListCustomersRequest{PageToken: "garbage"})
				return err
			},
			want: connect.CodeInvalidArgument,
		},
		{
			name: "list rejects a malformed email filter",
			call: func(c customerv1connect.CustomerServiceClient) error {
				_, err := c.ListCustomers(t.Context(), &customerv1.ListCustomersRequest{Email: "not-an-email"})
				return err
			},
			want: connect.CodeInvalidArgument,
		},
		{
			name: "list rejects an undefined order",
			call: func(c customerv1connect.CustomerServiceClient) error {
				_, err := c.ListCustomers(t.Context(), &customerv1.ListCustomersRequest{OrderBy: 99})
				return err
			},
			want: connect.CodeInvalidArgument,
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
