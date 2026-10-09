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
	"strings"
	"syscall"
	"time"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
	connectcors "connectrpc.com/cors"
	"connectrpc.com/grpchealth/v2"
	"connectrpc.com/grpcreflect/v2"
	"connectrpc.com/validate"
	"connectrpc.com/vanguard"
	"github.com/rs/cors"

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
	corsOrigins := flag.String("cors-origins", "", "comma-separated origins allowed to call the API from a browser (\"*\" for any); empty disables CORS")
	reflection := flag.Bool("reflection", false, "serve gRPC server reflection (describes every compiled-in protobuf type; keep off on public APIs)")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	opts := options{CORSOrigins: splitList(*corsOrigins), Reflection: *reflection}
	if err := run(ctx, *addr, opts, logger); err != nil {
		logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func splitList(s string) []string {
	var out []string
	for item := range strings.SplitSeq(s, ",") {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}

// options are the server settings that come from flags.
type options struct {
	CORSOrigins []string
	Reflection  bool
}

// protocols enables HTTP/1.1 and unencrypted HTTP/2 (h2c). gRPC clients and
// the bidirectional reflection stream need HTTP/2; TLS is expected to be
// terminated in front of the service.
func protocols() *http.Protocols {
	var p http.Protocols
	p.SetHTTP1(true)
	p.SetUnencryptedHTTP2(true)
	return &p
}

func run(ctx context.Context, addr string, opts options, logger *slog.Logger) error {
	handler, err := newHandler(logger, opts)
	if err != nil {
		return err
	}
	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		Protocols:         protocols(),
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
// all protocols. Add new contexts here, and their service name to the health
// checker.
func newHandler(logger *slog.Logger, opts options) (http.Handler, error) {
	customers := customer.NewStore()
	invoices := invoice.NewStore(customers)

	server := connect.NewServer(validate.NewServerInterceptor())
	customerv1connect.RegisterCustomerServiceHandler(server, customerrpc.NewHandler(customers, logger))
	invoicev1connect.RegisterInvoiceServiceHandler(server, invoicerpc.NewHandler(invoices, logger))

	// Standard gRPC health checks: the overall server ("") and each service.
	grpchealth.Register(server, grpchealth.NewStaticChecker(
		customerv1connect.CustomerServiceName,
		invoicev1connect.InvoiceServiceName,
	))
	if opts.Reflection {
		grpcreflect.Register(server)
	}

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
	return withCORS(mux, opts.CORSOrigins), nil
}

// withCORS lets browsers on the given origins call every protocol. Without
// origins it returns h unchanged: CORS stays off unless explicitly enabled.
// Credentials are not allowed; revisit when authentication is added.
func withCORS(h http.Handler, origins []string) http.Handler {
	if len(origins) == 0 {
		return h
	}
	return cors.New(cors.Options{
		AllowedOrigins: origins,
		// PATCH and DELETE are used by the REST routes Vanguard serves.
		AllowedMethods: append(connectcors.AllowedMethods(), http.MethodPatch, http.MethodDelete),
		AllowedHeaders: connectcors.AllowedHeaders(),
		ExposedHeaders: connectcors.ExposedHeaders(),
		MaxAge:         7200,
	}).Handler(h)
}
