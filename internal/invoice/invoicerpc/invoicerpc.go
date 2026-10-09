// Package invoicerpc exposes the invoice domain over Connect. It is the only
// place where generated contract types and domain types meet.
package invoicerpc

import (
	"context"
	"errors"
	"log/slog"
	"uuid"

	"connectrpc.com/connect/v2"
	"google.golang.org/protobuf/types/known/timestamppb"

	invoicev1 "github.com/telemac/connect-service-template/gen/go/invoice/v1"
	"github.com/telemac/connect-service-template/gen/go/invoice/v1/invoicev1connect"
	"github.com/telemac/connect-service-template/internal/invoice"
	"github.com/telemac/connect-service-template/internal/pagetoken"
)

const (
	defaultPageSize = 1000
	maxPageSize     = 10000
)

type Handler struct {
	invoicev1connect.UnimplementedInvoiceServiceHandler
	store  *invoice.Store
	logger *slog.Logger
}

func NewHandler(store *invoice.Store, logger *slog.Logger) *Handler {
	return &Handler{store: store, logger: logger}
}

func (h *Handler) CreateInvoice(ctx context.Context, req *invoicev1.CreateInvoiceRequest) (*invoicev1.CreateInvoiceResponse, error) {
	in := req.GetInvoice()
	customerID, err := parseID(in.GetCustomerId(), "invoice.customer_id")
	if err != nil {
		return nil, err
	}
	// id, status and timestamps are output only: the domain sets them.
	inv, err := h.store.Create(ctx, customerID, in.GetAmountCents(), in.GetCurrency())
	if err != nil {
		return nil, h.toConnectError(ctx, err)
	}
	return &invoicev1.CreateInvoiceResponse{Invoice: toContract(inv)}, nil
}

func (h *Handler) GetInvoice(ctx context.Context, req *invoicev1.GetInvoiceRequest) (*invoicev1.GetInvoiceResponse, error) {
	id, err := parseID(req.GetId(), "id")
	if err != nil {
		return nil, err
	}
	inv, err := h.store.Get(ctx, id)
	if err != nil {
		return nil, h.toConnectError(ctx, err)
	}
	return &invoicev1.GetInvoiceResponse{Invoice: toContract(inv)}, nil
}

func (h *Handler) ListInvoices(ctx context.Context, req *invoicev1.ListInvoicesRequest) (*invoicev1.ListInvoicesResponse, error) {
	var filter invoice.Filter
	if req.GetCustomerId() != "" {
		id, err := parseID(req.GetCustomerId(), "customer_id")
		if err != nil {
			return nil, err
		}
		filter.CustomerID = id
	}
	if req.GetStatus() != invoicev1.InvoiceStatus_INVOICE_STATUS_UNSPECIFIED {
		status, err := toStatus(req.GetStatus())
		if err != nil {
			return nil, err
		}
		filter.Status = status
	}

	after := uuid.Nil()
	if req.GetPageToken() != "" {
		var err error
		if after, err = pagetoken.Decode[invoice.Filter, uuid.UUID](req.GetPageToken(), filter); err != nil {
			return nil, pageTokenError(err)
		}
	}
	size := int(req.GetPageSize())
	switch {
	case size == 0:
		size = defaultPageSize
	case size > maxPageSize:
		size = maxPageSize
	}

	page, next, err := h.store.List(ctx, filter, after, size)
	if err != nil {
		return nil, h.toConnectError(ctx, err)
	}
	resp := &invoicev1.ListInvoicesResponse{Invoices: make([]*invoicev1.Invoice, 0, len(page))}
	for _, inv := range page {
		resp.Invoices = append(resp.Invoices, toContract(inv))
	}
	if next != uuid.Nil() {
		if resp.NextPageToken, err = pagetoken.Encode(filter, next); err != nil {
			return nil, h.toConnectError(ctx, err)
		}
	}
	return resp, nil
}

func (h *Handler) IssueInvoice(ctx context.Context, req *invoicev1.IssueInvoiceRequest) (*invoicev1.IssueInvoiceResponse, error) {
	inv, err := h.transition(ctx, req.GetId(), h.store.Issue)
	if err != nil {
		return nil, err
	}
	return &invoicev1.IssueInvoiceResponse{Invoice: inv}, nil
}

func (h *Handler) PayInvoice(ctx context.Context, req *invoicev1.PayInvoiceRequest) (*invoicev1.PayInvoiceResponse, error) {
	inv, err := h.transition(ctx, req.GetId(), h.store.Pay)
	if err != nil {
		return nil, err
	}
	return &invoicev1.PayInvoiceResponse{Invoice: inv}, nil
}

func (h *Handler) VoidInvoice(ctx context.Context, req *invoicev1.VoidInvoiceRequest) (*invoicev1.VoidInvoiceResponse, error) {
	inv, err := h.transition(ctx, req.GetId(), h.store.Void)
	if err != nil {
		return nil, err
	}
	return &invoicev1.VoidInvoiceResponse{Invoice: inv}, nil
}

func (h *Handler) transition(ctx context.Context, rawID string, apply func(context.Context, uuid.UUID) (invoice.Invoice, error)) (*invoicev1.Invoice, error) {
	id, err := parseID(rawID, "id")
	if err != nil {
		return nil, err
	}
	inv, err := apply(ctx, id)
	if err != nil {
		return nil, h.toConnectError(ctx, err)
	}
	return toContract(inv), nil
}

func parseID(raw, field string) (uuid.UUID, error) {
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil(), connect.Errorf(connect.CodeInvalidArgument, "%s must be a UUID", field)
	}
	return id, nil
}

func toContract(inv invoice.Invoice) *invoicev1.Invoice {
	return &invoicev1.Invoice{
		Id:          inv.ID.String(),
		CustomerId:  inv.CustomerID.String(),
		AmountCents: inv.AmountCents,
		Currency:    inv.Currency,
		Status:      fromStatus(inv.Status),
		CreateTime:  timestamppb.New(inv.CreateTime),
		UpdateTime:  timestamppb.New(inv.UpdateTime),
	}
}

func fromStatus(s invoice.Status) invoicev1.InvoiceStatus {
	switch s {
	case invoice.Draft:
		return invoicev1.InvoiceStatus_INVOICE_STATUS_DRAFT
	case invoice.Issued:
		return invoicev1.InvoiceStatus_INVOICE_STATUS_ISSUED
	case invoice.Paid:
		return invoicev1.InvoiceStatus_INVOICE_STATUS_PAID
	case invoice.Void:
		return invoicev1.InvoiceStatus_INVOICE_STATUS_VOID
	default:
		return invoicev1.InvoiceStatus_INVOICE_STATUS_UNSPECIFIED
	}
}

func toStatus(s invoicev1.InvoiceStatus) (invoice.Status, error) {
	switch s {
	case invoicev1.InvoiceStatus_INVOICE_STATUS_DRAFT:
		return invoice.Draft, nil
	case invoicev1.InvoiceStatus_INVOICE_STATUS_ISSUED:
		return invoice.Issued, nil
	case invoicev1.InvoiceStatus_INVOICE_STATUS_PAID:
		return invoice.Paid, nil
	case invoicev1.InvoiceStatus_INVOICE_STATUS_VOID:
		return invoice.Void, nil
	default:
		return 0, connect.Errorf(connect.CodeInvalidArgument, "unsupported status %v", s)
	}
}

func pageTokenError(err error) error {
	if errors.Is(err, pagetoken.ErrMismatch) {
		return connect.NewError(connect.CodeInvalidArgument, "page_token was issued for different filters").WithCause(err)
	}
	return connect.NewError(connect.CodeInvalidArgument, "invalid page_token").WithCause(err)
}

// toConnectError decides what callers may see. Unknown errors are logged here
// and reach the client as a bare internal code, never with their message.
func (h *Handler) toConnectError(ctx context.Context, err error) error {
	switch {
	case errors.Is(err, invoice.ErrNotFound):
		return connect.NewError(connect.CodeNotFound, err.Error()).WithCause(err)
	case errors.Is(err, invoice.ErrInvalid):
		return connect.NewError(connect.CodeInvalidArgument, err.Error()).WithCause(err)
	case errors.Is(err, invoice.ErrUnknownCustomer), errors.Is(err, invoice.ErrTransition):
		// The request is well formed but the system state forbids it.
		return connect.NewError(connect.CodeFailedPrecondition, err.Error()).WithCause(err)
	default:
		h.logger.ErrorContext(ctx, "invoice request failed", "error", err)
		return connect.NewError(connect.CodeInternal, "internal error").WithCause(err)
	}
}
