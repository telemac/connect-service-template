package invoicerpc_test

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

	invoicev1 "github.com/telemac/connectrpc-v2-poc/gen/go/invoice/v1"
	"github.com/telemac/connectrpc-v2-poc/gen/go/invoice/v1/invoicev1connect"
	"github.com/telemac/connectrpc-v2-poc/internal/customer"
	"github.com/telemac/connectrpc-v2-poc/internal/invoice"
	"github.com/telemac/connectrpc-v2-poc/internal/invoice/invoicerpc"
)

// newClient wires the real customer store, as cmd/server does, and returns
// the ID of one existing customer.
func newClient(t *testing.T) (invoicev1connect.InvoiceServiceClient, string) {
	t.Helper()
	customers := customer.NewStore()
	c, err := customers.Create(t.Context(), "Ada", "ada@example.com")
	require.NoError(t, err)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := connect.NewServer(validate.NewServerInterceptor())
	invoicev1connect.RegisterInvoiceServiceHandler(server, invoicerpc.NewHandler(invoice.NewStore(customers), logger))
	return invoicev1connect.NewInvoiceServiceClient(connect.NewClient(connectinprocess.New(server))), c.ID.String()
}

func codeOf(err error) connect.Code {
	if connectErr, ok := errors.AsType[*connect.Error](err); ok {
		return connectErr.Code()
	}
	return connect.CodeUnknown
}

func create(t *testing.T, client invoicev1connect.InvoiceServiceClient, customerID string) *invoicev1.Invoice {
	t.Helper()
	resp, err := client.CreateInvoice(t.Context(), &invoicev1.CreateInvoiceRequest{Invoice: &invoicev1.Invoice{
		CustomerId: customerID, AmountCents: 1250, Currency: "EUR",
	}})
	require.NoError(t, err)
	return resp.GetInvoice()
}

func TestCreatedInvoiceIsDraftAndIgnoresOutputOnlyFields(t *testing.T) {
	assert := assert.New(t)
	client, customerID := newClient(t)

	resp, err := client.CreateInvoice(t.Context(), &invoicev1.CreateInvoiceRequest{Invoice: &invoicev1.Invoice{
		CustomerId:  customerID,
		AmountCents: 1250,
		Currency:    "EUR",
		Status:      invoicev1.InvoiceStatus_INVOICE_STATUS_PAID,
	}})
	assert.NoError(err)
	assert.Equal(invoicev1.InvoiceStatus_INVOICE_STATUS_DRAFT, resp.GetInvoice().GetStatus())
	assert.NotEmpty(resp.GetInvoice().GetId())
	assert.NotNil(resp.GetInvoice().GetCreateTime())
}

func TestLifecycleThroughCustomMethods(t *testing.T) {
	assert := assert.New(t)
	client, customerID := newClient(t)
	id := create(t, client, customerID).GetId()

	issued, err := client.IssueInvoice(t.Context(), &invoicev1.IssueInvoiceRequest{Id: id})
	assert.NoError(err)
	assert.Equal(invoicev1.InvoiceStatus_INVOICE_STATUS_ISSUED, issued.GetInvoice().GetStatus())

	paid, err := client.PayInvoice(t.Context(), &invoicev1.PayInvoiceRequest{Id: id})
	assert.NoError(err)
	assert.Equal(invoicev1.InvoiceStatus_INVOICE_STATUS_PAID, paid.GetInvoice().GetStatus())

	_, err = client.VoidInvoice(t.Context(), &invoicev1.VoidInvoiceRequest{Id: id})
	assert.Equal(connect.CodeFailedPrecondition, codeOf(err), "a paid invoice cannot be voided")
}

func TestListInvoicesFiltersByCustomerAndStatus(t *testing.T) {
	assert := assert.New(t)
	client, customerID := newClient(t)
	first := create(t, client, customerID)
	create(t, client, customerID)
	_, err := client.IssueInvoice(t.Context(), &invoicev1.IssueInvoiceRequest{Id: first.GetId()})
	require.NoError(t, err)

	resp, err := client.ListInvoices(t.Context(), &invoicev1.ListInvoicesRequest{
		CustomerId: customerID,
		Status:     invoicev1.InvoiceStatus_INVOICE_STATUS_ISSUED,
	})
	assert.NoError(err)
	if assert.Len(resp.GetInvoices(), 1) {
		assert.Equal(first.GetId(), resp.GetInvoices()[0].GetId())
	}

	page, err := client.ListInvoices(t.Context(), &invoicev1.ListInvoicesRequest{PageSize: 1, CustomerId: customerID})
	require.NoError(t, err)
	_, err = client.ListInvoices(t.Context(), &invoicev1.ListInvoicesRequest{PageSize: 1, PageToken: page.GetNextPageToken()})
	assert.Equal(connect.CodeInvalidArgument, codeOf(err), "a token is bound to its filters")
}

func TestErrorsMapToConnectCodes(t *testing.T) {
	tests := []struct {
		name string
		call func(invoicev1connect.InvoiceServiceClient, string) error
		want connect.Code
	}{
		{
			name: "create requires the currency",
			call: func(c invoicev1connect.InvoiceServiceClient, customerID string) error {
				_, err := c.CreateInvoice(t.Context(), &invoicev1.CreateInvoiceRequest{Invoice: &invoicev1.Invoice{CustomerId: customerID, AmountCents: 100}})
				return err
			},
			want: connect.CodeInvalidArgument,
		},
		{
			name: "create rejects a negative amount",
			call: func(c invoicev1connect.InvoiceServiceClient, customerID string) error {
				_, err := c.CreateInvoice(t.Context(), &invoicev1.CreateInvoiceRequest{Invoice: &invoicev1.Invoice{CustomerId: customerID, AmountCents: -1, Currency: "EUR"}})
				return err
			},
			want: connect.CodeInvalidArgument,
		},
		{
			name: "create rejects a malformed currency",
			call: func(c invoicev1connect.InvoiceServiceClient, customerID string) error {
				_, err := c.CreateInvoice(t.Context(), &invoicev1.CreateInvoiceRequest{Invoice: &invoicev1.Invoice{CustomerId: customerID, AmountCents: 100, Currency: "euro"}})
				return err
			},
			want: connect.CodeInvalidArgument,
		},
		{
			name: "create for an unknown customer is a failed precondition",
			call: func(c invoicev1connect.InvoiceServiceClient, _ string) error {
				_, err := c.CreateInvoice(t.Context(), &invoicev1.CreateInvoiceRequest{Invoice: &invoicev1.Invoice{CustomerId: uuid.NewV7().String(), AmountCents: 100, Currency: "EUR"}})
				return err
			},
			want: connect.CodeFailedPrecondition,
		},
		{
			name: "paying a draft is a failed precondition",
			call: func(c invoicev1connect.InvoiceServiceClient, customerID string) error {
				_, err := c.PayInvoice(t.Context(), &invoicev1.PayInvoiceRequest{Id: create(t, c, customerID).GetId()})
				return err
			},
			want: connect.CodeFailedPrecondition,
		},
		{
			name: "unknown invoice is not found",
			call: func(c invoicev1connect.InvoiceServiceClient, _ string) error {
				_, err := c.IssueInvoice(t.Context(), &invoicev1.IssueInvoiceRequest{Id: uuid.NewV7().String()})
				return err
			},
			want: connect.CodeNotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert := assert.New(t)
			client, customerID := newClient(t)
			assert.Equal(tt.want, codeOf(tt.call(client, customerID)))
		})
	}
}
