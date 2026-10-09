// Command server serves every API context over Connect, gRPC, gRPC-Web and
// REST, plus the OpenAPI document. It is the only place where dependencies
// are built and wired together.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
	"connectrpc.com/validate"
	"connectrpc.com/vanguard"

	"github.com/telemac/connectrpc-v2-poc/gen/go/customer/v1/customerv1connect"
	"github.com/telemac/connectrpc-v2-poc/gen/go/invoice/v1/invoicev1connect"
	"github.com/telemac/connectrpc-v2-poc/internal/customer"
	"github.com/telemac/connectrpc-v2-poc/internal/customer/customerrpc"
	"github.com/telemac/connectrpc-v2-poc/internal/invoice"
	"github.com/telemac/connectrpc-v2-poc/internal/invoice/invoicerpc"
	"github.com/telemac/connectrpc-v2-poc/internal/openapi"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, *addr, logger); err != nil {
		logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, addr string, logger *slog.Logger) error {
	handler, err := newHandler(logger)
	if err != nil {
		return err
	}
	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	errc := make(chan error, 1)
	go func() {
		logger.Info("listening", "addr", addr)
		errc <- srv.ListenAndServe()
	}()

	select {
	case err := <-errc:
		return fmt.Errorf("serving http: %w", err)
	case <-ctx.Done():
	}

	// The parent context is already cancelled; shutdown needs its own deadline.
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutting down: %w", err)
	}
	if err := <-errc; !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serving http: %w", err)
	}
	return nil
}

// newHandler wires every context onto one connect.Server and exposes it over
// all protocols. Add new contexts here.
func newHandler(logger *slog.Logger) (http.Handler, error) {
	customers := customer.NewStore()
	invoices := invoice.NewStore(customers)

	server := connect.NewServer(validate.NewServerInterceptor())
	customerv1connect.RegisterCustomerServiceHandler(server, customerrpc.NewHandler(customers, logger))
	invoicev1connect.RegisterInvoiceServiceHandler(server, invoicerpc.NewHandler(invoices, logger))

	mux := http.NewServeMux()
	connecthttp.Mount(mux, server)
	// Vanguard installs a catch-all "/" route; more specific routes still win.
	if err := vanguard.Mount(mux, server); err != nil {
		return nil, fmt.Errorf("mounting rest routes: %w", err)
	}
	mux.HandleFunc("GET /openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		if _, err := w.Write(openapi.Document); err != nil {
			logger.WarnContext(r.Context(), "writing openapi document", "error", err)
		}
	})
	return mux, nil
}
