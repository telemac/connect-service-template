package pagetoken_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/telemac/connectrpc-v2-poc/internal/pagetoken"
)

type params struct {
	Name string
	Desc bool
}

type cursor struct {
	Key string
	N   int
}

func TestRoundTripReturnsCursor(t *testing.T) {
	assert := assert.New(t)
	tok, err := pagetoken.Encode(params{Name: "ada"}, cursor{Key: "k", N: 7})
	assert.NoError(err)

	got, err := pagetoken.Decode[params, cursor](tok, params{Name: "ada"})
	assert.NoError(err)
	assert.Equal(cursor{Key: "k", N: 7}, got)
}

func TestDecodeRejects(t *testing.T) {
	tok, err := pagetoken.Encode(params{Name: "ada"}, cursor{N: 1})
	require.NoError(t, err)

	tests := []struct {
		name   string
		token  string
		params params
		want   error
	}{
		{"other parameters", tok, params{Name: "ada", Desc: true}, pagetoken.ErrMismatch},
		{"not base64", "%%%", params{Name: "ada"}, pagetoken.ErrMalformed},
		{"not json", "Z2FyYmFnZQ", params{Name: "ada"}, pagetoken.ErrMalformed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert := assert.New(t)
			_, err := pagetoken.Decode[params, cursor](tt.token, tt.params)
			assert.ErrorIs(err, tt.want)
		})
	}
}
