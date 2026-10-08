// Package openapi embeds the OpenAPI document that make generate writes next
// to this file from the protos in api/.
package openapi

import _ "embed"

//go:embed openapi.yaml
var Document []byte
