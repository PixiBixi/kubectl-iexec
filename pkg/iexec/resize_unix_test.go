//go:build !windows

package iexec

import (
	"testing"

	"k8s.io/client-go/tools/remotecommand"
)

func TestPushLatestKeepsNewestSize(t *testing.T) {
	q := make(sizeQueue, 1)
	pushLatest(q, remotecommand.TerminalSize{Width: 80, Height: 24})
	pushLatest(q, remotecommand.TerminalSize{Width: 120, Height: 40})
	if got := <-q; got.Width != 120 || got.Height != 40 {
		t.Fatalf("want the newest size 120x40, got %dx%d", got.Width, got.Height)
	}
	select {
	case extra := <-q:
		t.Fatalf("stale size left in queue: %v", extra)
	default:
	}
}
