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

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	handler, err := newHandler(slog.New(slog.NewTextHandler(io.Discard, nil)))
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

	// REST PUT binds {id} from the path and the body to customer.
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPut, srv.URL+"/v1/customers/"+id,
		strings.NewReader(`{"name":"Ada Lovelace","email":"ada@lovelace.org"}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	putResp, err := srv.Client().Do(req)
	require.NoError(t, err)
	defer putResp.Body.Close()
	assert.Equal(http.StatusOK, putResp.StatusCode)

	got, err := client.GetCustomer(t.Context(), &customerv1.GetCustomerRequest{Id: id})
	require.NoError(t, err)
	assert.Equal("Ada Lovelace", got.GetCustomer().GetName())
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
