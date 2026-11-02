package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/PixiBixi/kubectl-iexec/v2/pkg/cmd"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	utilexec "k8s.io/client-go/util/exec"
)

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	flags := pflag.NewFlagSet("kubectl-iexec", pflag.ExitOnError)
	pflag.CommandLine = flags

	root := cmd.NewCmdIExec(genericclioptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr})
	// kubectl >= 1.26 completes plugins through kubectl_complete-iexec in PATH:
	// a symlink to this binary under that name answers cobra's __complete.
	if strings.HasPrefix(filepath.Base(os.Args[0]), "kubectl_complete-") {
		root.SetArgs(append([]string{cobra.ShellCompRequestCmd}, os.Args[1:]...))
	}
	// Cancels the pod List and the exec stream on Ctrl-C when no TTY is attached.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := exitCode(root.ExecuteContext(ctx), os.Stderr)
	stop()
	os.Exit(code)
}

// exitCode returns the remote command's exit status as is, like kubectl exec,
// 130 when interrupted, and prints any other error on stderr with status 1.
func exitCode(err error, stderr io.Writer) int {
	if err == nil {
		return 0
	}
	if errors.Is(err, context.Canceled) {
		return 130 // interrupted, like a shell after Ctrl-C
	}
	var exitErr utilexec.ExitError
	if errors.As(err, &exitErr) && exitErr.Exited() {
		return exitErr.ExitStatus()
	}
	_, _ = fmt.Fprintln(stderr, "Error:", err)
	return 1
}
