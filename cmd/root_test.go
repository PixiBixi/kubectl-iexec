package cmd

import (
	"bytes"
	"context"
	"testing"

	"github.com/pkg/errors"
	utilexec "k8s.io/client-go/util/exec"
)

func TestExitCode(t *testing.T) {
	remote := errors.Wrap(errors.Wrap(utilexec.CodeExitError{Err: errors.New("command terminated with exit code 3"), Code: 3}, "exec stream failed"), "RunExec failed")

	tests := []struct {
		name       string
		err        error
		want       int
		wantStderr string
	}{
		{"success", nil, 0, ""},
		{"remote exit code through wraps", remote, 3, ""},
		{"interrupted", errors.Wrap(context.Canceled, "exec stream failed"), 130, ""},
		{"local error", errors.New("no pods found for filter: zzz"), 1, "Error: no pods found for filter: zzz\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stderr bytes.Buffer
			if got := exitCode(tt.err, &stderr); got != tt.want {
				t.Errorf("exitCode() = %d, want %d", got, tt.want)
			}
			if stderr.String() != tt.wantStderr {
				t.Errorf("stderr = %q, want %q", stderr.String(), tt.wantStderr)
			}
		})
	}
}
