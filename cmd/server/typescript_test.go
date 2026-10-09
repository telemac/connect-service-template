package main

import (
	"os"
	"os/exec"
	"testing"
)

// TestTypeScriptClient runs the TypeScript client tests (ts/src/*.test.ts)
// against this server, so the generated client, its transport and the
// client-side validation are checked end to end by go test and make check.
func TestTypeScriptClient(t *testing.T) {
	if testing.Short() {
		t.Skip("starts npm; skipped with -short")
	}
	if _, err := os.Stat("../../ts/node_modules"); err != nil {
		t.Skip("ts/node_modules missing: run make setup")
	}
	srv := newTestServer(t, options{})

	cmd := exec.CommandContext(t.Context(), "npm", "--prefix", "../../ts", "test")
	cmd.Env = append(os.Environ(), "BASE_URL="+srv.URL)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("npm test failed: %v\n%s", err, out)
	}
}
