// Package customerrpc exposes the customer domain over Connect. It is the only
// place where generated contract types and domain types meet.
package customerrpc

import (
	"context"
	"errors"
	"log/slog"
	"uuid"

	"connectrpc.com/connect/v2"

	customerv1 "github.com/telemac/connectrpc-v2-poc/gen/go/customer/v1"
	"github.com/telemac/connectrpc-v2-poc/gen/go/customer/v1/customerv1connect"
	"github.com/telemac/connectrpc-v2-poc/internal/customer"
)

type Handler struct {
	customerv1connect.UnimplementedCustomerServiceHandler
	store  *customer.Store
	logger *slog.Logger
}

func NewHandler(store *customer.Store, logger *slog.Logger) *Handler {
	return &Handler{store: store, logger: logger}
}

func (h *Handler) CreateCustomer(ctx context.Context, req *customerv1.CreateCustomerRequest) (*customerv1.CreateCustomerResponse, error) {
	// The client-supplied id is ignored: the domain assigns it.
	c, err := h.store.Create(ctx, req.GetCustomer().GetName(), req.GetCustomer().GetEmail())
	if err != nil {
		return nil, h.toConnectError(ctx, err)
	}
	return &customerv1.CreateCustomerResponse{Customer: toContract(c)}, nil
}

func (h *Handler) GetCustomer(ctx context.Context, req *customerv1.GetCustomerRequest) (*customerv1.GetCustomerResponse, error) {
	id, err := uuid.Parse(req.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, "id must be a UUID")
	}
	c, err := h.store.Get(ctx, id)
	if err != nil {
		return nil, h.toConnectError(ctx, err)
	}
	return &customerv1.GetCustomerResponse{Customer: toContract(c)}, nil
}

func (h *Handler) UpdateCustomer(ctx context.Context, req *customerv1.UpdateCustomerRequest) (*customerv1.UpdateCustomerResponse, error) {
	id, err := uuid.Parse(req.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, "id must be a UUID")
	}
	c, err := h.store.Update(ctx, id, req.GetCustomer().GetName(), req.GetCustomer().GetEmail())
	if err != nil {
		return nil, h.toConnectError(ctx, err)
	}
	return &customerv1.UpdateCustomerResponse{Customer: toContract(c)}, nil
}

func (h *Handler) DeleteCustomer(ctx context.Context, req *customerv1.DeleteCustomerRequest) (*customerv1.DeleteCustomerResponse, error) {
	id, err := uuid.Parse(req.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, "id must be a UUID")
	}
	if err := h.store.Delete(ctx, id); err != nil {
		return nil, h.toConnectError(ctx, err)
	}
	return &customerv1.DeleteCustomerResponse{}, nil
}

func toContract(c customer.Customer) *customerv1.Customer {
	return &customerv1.Customer{Id: c.ID.String(), Name: c.Name, Email: c.Email}
}

// toConnectError decides what callers may see. Unknown errors are logged here
// and reach the client as a bare internal code, never with their message.
func (h *Handler) toConnectError(ctx context.Context, err error) error {
	switch {
	case errors.Is(err, customer.ErrNotFound):
		return connect.NewError(connect.CodeNotFound, err.Error()).WithCause(err)
	case errors.Is(err, customer.ErrInvalid):
		return connect.NewError(connect.CodeInvalidArgument, err.Error()).WithCause(err)
	default:
		h.logger.ErrorContext(ctx, "customer request failed", "error", err)
		return connect.NewError(connect.CodeInternal, "internal error").WithCause(err)
	}
}
