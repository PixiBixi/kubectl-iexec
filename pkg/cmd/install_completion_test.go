package cmd

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestLinkCompletion(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "kubectl-iexec")
	link := filepath.Join(dir, completionLinkName)
	if err := os.WriteFile(bin, nil, 0o755); err != nil { //nolint:gosec // test binary must be executable
		t.Fatal(err)
	}

	for range 2 { // second run is a no-op
		if err := linkCompletion(bin, io.Discard); err != nil {
			t.Fatalf("linkCompletion: %v", err)
		}
		if target, err := os.Readlink(link); err != nil || target != bin {
			t.Fatalf("link -> %q, %v; want %q", target, err, bin)
		}
	}

	// A stale link to an older kubectl-iexec is replaced.
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/old/path/kubectl-iexec", link); err != nil {
		t.Fatal(err)
	}
	if err := linkCompletion(bin, io.Discard); err != nil {
		t.Fatalf("stale link: %v", err)
	}
	if target, _ := os.Readlink(link); target != bin {
		t.Errorf("stale link not replaced: %q", target)
	}
}

func TestLinkCompletion_KeepsForeignFiles(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "kubectl-iexec")
	link := filepath.Join(dir, completionLinkName)

	if err := os.WriteFile(link, []byte("#!/bin/sh\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := linkCompletion(bin, io.Discard); err == nil {
		t.Error("a regular file must not be replaced")
	}

	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/usr/local/bin/something-else", link); err != nil {
		t.Fatal(err)
	}
	if err := linkCompletion(bin, io.Discard); err == nil {
		t.Error("a symlink to another program must not be replaced")
	}
}
