package main

import (
	"io"
	"log/slog"
	"strings"
	"testing"
)

func TestRunRejectsMissingConfiguration(t *testing.T) {
	t.Setenv("HELLOASSO_API_ID", "")
	err := run(slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err == nil || !strings.Contains(err.Error(), "HELLOASSO_API_ID") {
		t.Fatalf("err=%v", err)
	}
}
