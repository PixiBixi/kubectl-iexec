package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/PixiBixi/kubectl-iexec/v2/pkg/cmd"
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
	os.Exit(exitCode(root.Execute(), os.Stderr))
}

// exitCode returns the remote command's exit status as is, like kubectl exec,
// and prints any other error on stderr with status 1.
func exitCode(err error, stderr io.Writer) int {
	if err == nil {
		return 0
	}
	var exitErr utilexec.ExitError
	if errors.As(err, &exitErr) && exitErr.Exited() {
		return exitErr.ExitStatus()
	}
	_, _ = fmt.Fprintln(stderr, "Error:", err)
	return 1
}
