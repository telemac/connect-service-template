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
	"github.com/telemac/connectrpc-v2-poc/internal/pagetoken"
)

const (
	defaultPageSize = 1000
	maxPageSize     = 10000
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

func (h *Handler) ListCustomers(ctx context.Context, req *customerv1.ListCustomersRequest) (*customerv1.ListCustomersResponse, error) {
	filter := customer.Filter{NameContains: req.GetNameContains(), Email: req.GetEmail()}
	order, err := toOrder(req.GetOrderBy(), req.GetDescending())
	if err != nil {
		return nil, err
	}
	params := listParams{Filter: filter, Order: order}
	var after customer.Cursor
	if req.GetPageToken() != "" {
		if after, err = pagetoken.Decode[listParams, customer.Cursor](req.GetPageToken(), params); err != nil {
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

	page, next, err := h.store.List(ctx, filter, order, after, size)
	if err != nil {
		return nil, h.toConnectError(ctx, err)
	}
	resp := &customerv1.ListCustomersResponse{Customers: make([]*customerv1.Customer, 0, len(page))}
	for _, c := range page {
		resp.Customers = append(resp.Customers, toContract(c))
	}
	if next != (customer.Cursor{}) {
		if resp.NextPageToken, err = pagetoken.Encode(params, next); err != nil {
			return nil, h.toConnectError(ctx, err)
		}
	}
	return resp, nil
}

// listParams is what a page token is bound to.
type listParams struct {
	Filter customer.Filter
	Order  customer.Order
}

func pageTokenError(err error) error {
	if errors.Is(err, pagetoken.ErrMismatch) {
		return connect.NewError(connect.CodeInvalidArgument, "page_token was issued for different filters or order").WithCause(err)
	}
	return connect.NewError(connect.CodeInvalidArgument, "invalid page_token").WithCause(err)
}

func toOrder(by customerv1.CustomerOrder, descending bool) (customer.Order, error) {
	order := customer.Order{Descending: descending}
	switch by {
	case customerv1.CustomerOrder_CUSTOMER_ORDER_UNSPECIFIED:
		order.Key = customer.SortByCreation
	case customerv1.CustomerOrder_CUSTOMER_ORDER_NAME:
		order.Key = customer.SortByName
	case customerv1.CustomerOrder_CUSTOMER_ORDER_EMAIL:
		order.Key = customer.SortByEmail
	default:
		return customer.Order{}, connect.Errorf(connect.CodeInvalidArgument, "unsupported order_by %v", by)
	}
	return order, nil
}

func (h *Handler) UpdateCustomer(ctx context.Context, req *customerv1.UpdateCustomerRequest) (*customerv1.UpdateCustomerResponse, error) {
	id, err := uuid.Parse(req.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, "id must be a UUID")
	}
	c, err := h.store.Update(ctx, id, toChange(req.GetCustomer(), req.GetUpdateMask().GetPaths()))
	if err != nil {
		return nil, h.toConnectError(ctx, err)
	}
	return &customerv1.UpdateCustomerResponse{Customer: toContract(c)}, nil
}

// toChange applies AIP-134 mask semantics. Paths were already restricted to
// known ones by the contract.
func toChange(c *customerv1.Customer, paths []string) customer.Change {
	var change customer.Change
	if len(paths) == 0 {
		// An omitted mask means every populated field.
		if c.GetName() != "" {
			change.Name = new(c.GetName())
		}
		if c.GetEmail() != "" {
			change.Email = new(c.GetEmail())
		}
		return change
	}
	for _, path := range paths {
		switch path {
		case "*":
			change.Name, change.Email = new(c.GetName()), new(c.GetEmail())
		case "name":
			change.Name = new(c.GetName())
		case "email":
			change.Email = new(c.GetEmail())
		}
		// "id" is output only: ignored in masks (AIP-203).
	}
	return change
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
