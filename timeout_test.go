package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

func TestGoCommandPreservesCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := goCommand(ctx, t.TempDir(), nil, "env", "GOVERSION")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation cause, got %v", err)
	}
}

func TestTimeoutDiagnosticAndRecovery(t *testing.T) {
	dir, _ := fixture(t, "assets")
	var out, stderr bytes.Buffer
	if code := run([]string{"scan", "--dir", dir, "--json", "--timeout", "1ns"}, &out, &stderr); code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
	if out.Len() != 0 || !strings.Contains(stderr.String(), "context deadline exceeded") || !strings.Contains(stderr.String(), "--timeout 5m") {
		t.Fatalf("expected no report and actionable deadline error; stdout=%q stderr=%q", &out, &stderr)
	}
	out.Reset()
	stderr.Reset()
	if code := run([]string{"scan", "--dir", dir, "--timeout", "5m"}, &out, &stderr); code != 0 {
		t.Fatalf("retry exit %d: %s", code, &stderr)
	}
}

func TestInvalidTimeout(t *testing.T) {
	for _, duration := range []string{"0s", "-1s", "2h", "invalid"} {
		t.Run(duration, func(t *testing.T) {
			var out, stderr bytes.Buffer
			if code := run([]string{"scan", "--timeout", duration}, &out, &stderr); code != 2 || out.Len() != 0 {
				t.Fatalf("exit %d, stdout=%q, stderr=%q", code, &out, &stderr)
			}
		})
	}
}
