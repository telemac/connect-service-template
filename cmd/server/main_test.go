package main

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	customerv1 "github.com/telemac/connectrpc-v2-poc/gen/go/customer/v1"
	"github.com/telemac/connectrpc-v2-poc/gen/go/customer/v1/customerv1connect"
)

func newTestServer(t *testing.T, corsOrigins ...string) *httptest.Server {
	t.Helper()
	handler, err := newHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), corsOrigins)
	require.NoError(t, err)
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}

func TestCustomerIsReachableOverConnectAndREST(t *testing.T) {
	assert := assert.New(t)
	srv := newTestServer(t)

	// Connect protocol, binary proto codec.
	client := customerv1connect.NewCustomerServiceClient(connect.NewClient(connecthttp.NewTransport(srv.Client(), srv.URL)))
	created, err := client.CreateCustomer(t.Context(), &customerv1.CreateCustomerRequest{Customer: &customerv1.Customer{Name: "Ada", Email: "ada@example.com"}})
	require.NoError(t, err)
	id := created.GetCustomer().GetId()

	// REST, from the google.api.http annotations.
	resp, err := srv.Client().Get(srv.URL + "/v1/customers/" + id)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(http.StatusOK, resp.StatusCode)

	var body struct {
		Customer struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"customer"`
	}
	assert.NoError(json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(id, body.Customer.ID)
	assert.Equal("Ada", body.Customer.Name)

	// REST PATCH binds {id} from the path, update_mask from the query and the
	// body to customer.
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPatch, srv.URL+"/v1/customers/"+id+"?update_mask=name",
		strings.NewReader(`{"name":"Ada Lovelace","email":"ignored@example.com"}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	putResp, err := srv.Client().Do(req)
	require.NoError(t, err)
	defer putResp.Body.Close()
	assert.Equal(http.StatusOK, putResp.StatusCode)

	got, err := client.GetCustomer(t.Context(), &customerv1.GetCustomerRequest{Id: id})
	require.NoError(t, err)
	assert.Equal("Ada Lovelace", got.GetCustomer().GetName())
	assert.Equal("ada@example.com", got.GetCustomer().GetEmail())

	// REST list, with query parameters mapped to request fields.
	listResp, err := srv.Client().Get(srv.URL + "/v1/customers?page_size=10&name_contains=ada&order_by=CUSTOMER_ORDER_NAME&descending=true")
	require.NoError(t, err)
	defer listResp.Body.Close()
	var list struct {
		Customers []struct {
			ID string `json:"id"`
		} `json:"customers"`
	}
	assert.NoError(json.NewDecoder(listResp.Body).Decode(&list))
	if assert.Len(list.Customers, 1) {
		assert.Equal(id, list.Customers[0].ID)
	}

	// REST DELETE, then the customer is gone.
	delReq, err := http.NewRequestWithContext(t.Context(), http.MethodDelete, srv.URL+"/v1/customers/"+id, nil)
	require.NoError(t, err)
	delResp, err := srv.Client().Do(delReq)
	require.NoError(t, err)
	defer delResp.Body.Close()
	assert.Equal(http.StatusOK, delResp.StatusCode)

	goneResp, err := srv.Client().Get(srv.URL + "/v1/customers/" + id)
	require.NoError(t, err)
	defer goneResp.Body.Close()
	assert.Equal(http.StatusNotFound, goneResp.StatusCode)
}

func TestRESTRejectsInvalidInput(t *testing.T) {
	assert := assert.New(t)
	srv := newTestServer(t)

	resp, err := srv.Client().Post(srv.URL+"/v1/customers", "application/json", strings.NewReader(`{"name":"","email":"x"}`))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(http.StatusBadRequest, resp.StatusCode)
}

func TestOpenAPIDocumentIsServed(t *testing.T) {
	assert := assert.New(t)
	srv := newTestServer(t)

	resp, err := srv.Client().Get(srv.URL + "/openapi.yaml")
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	assert.NoError(err)
	assert.Equal(http.StatusOK, resp.StatusCode)
	assert.Contains(string(body), "/v1/customers/{id}")
}

func TestInvoiceCustomMethodIsReachableOverREST(t *testing.T) {
	assert := assert.New(t)
	srv := newTestServer(t)
	post := func(path, body string) (*http.Response, map[string]any) {
		t.Helper()
		resp, err := srv.Client().Post(srv.URL+path, "application/json", strings.NewReader(body))
		require.NoError(t, err)
		defer resp.Body.Close()
		var out map[string]any
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
		return resp, out
	}

	_, c := post("/v1/customers", `{"name":"Ada","email":"ada@example.com"}`)
	customerID := c["customer"].(map[string]any)["id"].(string)
	_, inv := post("/v1/invoices", `{"customerId":"`+customerID+`","amountCents":"1250","currency":"EUR"}`)
	invoiceID := inv["invoice"].(map[string]any)["id"].(string)

	resp, issued := post("/v1/invoices/"+invoiceID+":issue", `{}`)
	assert.Equal(http.StatusOK, resp.StatusCode)
	assert.Equal("INVOICE_STATUS_ISSUED", issued["invoice"].(map[string]any)["status"])

	resp, _ = post("/v1/invoices/"+invoiceID+":issue", `{}`)
	assert.Equal(http.StatusBadRequest, resp.StatusCode, "FailedPrecondition maps to 400 over REST")
}

// Preflight header lists are sorted and lowercase, as browsers send them;
// rs/cors rejects other forms.
func TestCORSPreflight(t *testing.T) {
	const allowed = "https://app.example.com"
	tests := []struct {
		name        string
		corsOrigins []string
		origin      string
		method      string
		path        string
		headers     string
		wantAllowed bool
	}{
		{"connect call from an allowed origin", []string{allowed}, allowed, http.MethodPost, "/customer.v1.CustomerService/GetCustomer", "connect-protocol-version,content-type", true},
		{"rest patch from an allowed origin", []string{allowed}, allowed, http.MethodPatch, "/v1/customers/x", "content-type", true},
		{"other origin", []string{allowed}, "https://evil.example.com", http.MethodPost, "/customer.v1.CustomerService/GetCustomer", "content-type", false},
		{"cors disabled by default", nil, allowed, http.MethodPost, "/customer.v1.CustomerService/GetCustomer", "content-type", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert := assert.New(t)
			srv := newTestServer(t, tt.corsOrigins...)

			req, err := http.NewRequestWithContext(t.Context(), http.MethodOptions, srv.URL+tt.path, nil)
			require.NoError(t, err)
			req.Header.Set("Origin", tt.origin)
			req.Header.Set("Access-Control-Request-Method", tt.method)
			req.Header.Set("Access-Control-Request-Headers", tt.headers)
			resp, err := srv.Client().Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			if tt.wantAllowed {
				assert.Equal(tt.origin, resp.Header.Get("Access-Control-Allow-Origin"))
				assert.Contains(resp.Header.Get("Access-Control-Allow-Methods"), tt.method)
			} else {
				assert.Empty(resp.Header.Get("Access-Control-Allow-Origin"))
			}
		})
	}
}

func TestCORSExposesGRPCWebTrailersOnActualRequests(t *testing.T) {
	assert := assert.New(t)
	const allowed = "https://app.example.com"
	srv := newTestServer(t, allowed)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/openapi.yaml", nil)
	require.NoError(t, err)
	req.Header.Set("Origin", allowed)
	resp, err := srv.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(allowed, resp.Header.Get("Access-Control-Allow-Origin"))
	assert.Contains(resp.Header.Get("Access-Control-Expose-Headers"), "Grpc-Status")
}
