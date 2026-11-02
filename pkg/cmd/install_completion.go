package cmd

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/pkg/errors"
)

// completionLinkName is the executable kubectl (>= 1.26) runs to complete
// `kubectl iexec`; the binary answers __complete when invoked under it.
const completionLinkName = "kubectl_complete-iexec"

// installCompletion links kubectl_complete-iexec next to the kubectl-iexec
// found on PATH. The link targets that path, not the resolved file, so a
// krew upgrade (new version dir, same ~/.krew/bin symlink) keeps working.
func installCompletion(out io.Writer) error {
	bin, err := exec.LookPath("kubectl-iexec")
	if err != nil {
		if bin, err = os.Executable(); err != nil {
			return errors.Wrap(err, "unable to locate kubectl-iexec")
		}
	}
	bin, err = filepath.Abs(bin)
	if err != nil {
		return err
	}
	return linkCompletion(bin, out)
}

func linkCompletion(bin string, out io.Writer) error {
	link := filepath.Join(filepath.Dir(bin), completionLinkName)

	switch target, err := os.Readlink(link); {
	case err == nil && target == bin:
		_, _ = fmt.Fprintf(out, "%s already installed\n", link)
		return nil
	case err == nil:
		// Our own stale link (older install location): replace it.
		if filepath.Base(target) != "kubectl-iexec" {
			return fmt.Errorf("%s already links to %s, not replacing it", link, target)
		}
		if err := os.Remove(link); err != nil {
			return err
		}
	case !os.IsNotExist(err):
		if _, statErr := os.Lstat(link); statErr == nil {
			return fmt.Errorf("%s exists and is not a symlink, not replacing it", link)
		}
	}

	if err := os.Symlink(bin, link); err != nil {
		return errors.Wrapf(err, "unable to create %s", link)
	}
	_, _ = fmt.Fprintf(out, "Installed %s -> %s\nkubectl iexec <TAB> now completes (kubectl >= 1.26, kubectl completion enabled in your shell)\n", link, bin)
	return nil
}
