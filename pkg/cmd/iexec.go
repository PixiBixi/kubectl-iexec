package cmd

import (
	"context"
	"fmt"

	"k8s.io/client-go/kubernetes"

	"github.com/pkg/errors"

	"github.com/PixiBixi/kubectl-iexec/v2/pkg/iexec"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest"
)

var (
	iexecLong = `
IExec is an interactive pod and container selector for 'kubectl exec'

Arg[1] will act as a filter, any pods that match will be returned in a list
that the user can select from.

Arg[2...] are the commands to be executed in the container. Put -- before
a command that has its own flags, otherwise iexec parses them.
`
	iexecExample = `
	# select from all pods in the namespace then run: 'kubectl exec sh'
	%[1]s iexec

	# select from all pods matching [busybox] then run: 'kubectl exec [pod_name] /bin/sh'
	%[1]s iexec busybox

	# select from all pods matching [busybox] then run: 'kubectl exec [pod_name] cat /etc/hosts'
	%[1]s iexec busybox cat /etc/hosts

	# select from all pods matching [multi_container_pod]
	# then select from all containers in pod matching [second_container]
	# then run: 'kubectl exec [pod_name] [container_name] /bin/sh'
	%[1]s iexec multi_container_pod -c second_container

	# skip the prompts and run in the first matching pod and container
	%[1]s iexec busybox --first cat /etc/hosts

	# put -- before a command that has its own flags
	%[1]s iexec busybox -- sh -c 'echo $HOSTNAME'
`
)

// defaultShell runs bash when the image ships it and falls back to sh
// (alpine, busybox) instead of failing with `exec: "bash": not found`.
var defaultShell = []string{"/bin/sh", "-c", "command -v bash >/dev/null 2>&1 && exec bash || exec sh"}

// remoteCommand returns the command given after the pod filter, or defaultShell.
func remoteCommand(args []string) []string {
	if len(args) > 1 {
		return append([]string(nil), args[1:]...)
	}
	return defaultShell
}

// IExecOptions configures the Iexec runner
type IExecOptions struct {
	configFlags *genericclioptions.ConfigFlags
	clientCfg   *rest.Config

	allNamespaces   bool
	remoteCmd       []string
	containerFilter string
	lvl             string
	namespace       string
	labelSelector   string
	naked           bool
	vimMode         bool
	first           bool
	allPhases       bool
	debug           bool
	debugImage      string

	genericclioptions.IOStreams
}

// NewIExecOptions provides an instance of IExecOptions with default values.
func NewIExecOptions(streams genericclioptions.IOStreams) *IExecOptions {
	return &IExecOptions{
		configFlags: genericclioptions.NewConfigFlags(true),

		IOStreams: streams,
	}
}

// NewCmdIExec provides a cobra command wrapping IExecOptions.
func NewCmdIExec(streams genericclioptions.IOStreams) *cobra.Command {
	o := NewIExecOptions(streams)

	cmd := &cobra.Command{
		Use:           "iexec [pod filter] [remote command(s)] [flags]",
		Short:         "Interactive remote shell into a Kubernetes Pod",
		Example:       fmt.Sprintf(iexecExample, "kubectl"),
		Long:          iexecLong,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(c *cobra.Command, args []string) error {
			if err := o.Complete(c, args); err != nil {
				return err
			}
			if err := o.Run(c.Context(), args); err != nil {
				return err
			}

			return nil
		},
	}

	cmd.Flags().BoolVarP(&o.allNamespaces, "all-namespaces", "A", o.allNamespaces, "If present, list the requested object(s) across all namespaces. Namespace in current context is ignored even if specified with --namespace.")
	cmd.PersistentFlags().StringVarP(&o.containerFilter, "container", "c", "", "Container to search")
	cmd.PersistentFlags().StringVarP(&o.lvl, "log-level", "", "", "log level (trace|debug|info|warn|error|fatal|panic)")
	cmd.PersistentFlags().StringVarP(&o.labelSelector, "selector", "l", "", "Selector (label query) to filter on, supports '=', '==', and '!='.(e.g. -l key1=value1,key2=value2)")
	cmd.PersistentFlags().BoolVarP(&o.vimMode, "vim-mode", "v", false, "Vim Mode enabled")
	_ = cmd.PersistentFlags().MarkDeprecated("vim-mode", "j/k navigation is always on")
	cmd.PersistentFlags().BoolVarP(&o.naked, "naked", "x", false, "Decolorize output")
	cmd.PersistentFlags().BoolVarP(&o.first, "first", "", false, "Skip the prompts and use the first matching pod and container")
	cmd.PersistentFlags().BoolVarP(&o.allPhases, "all-phases", "", false, "Include pods that are not Running or are terminating")
	cmd.PersistentFlags().BoolVar(&o.debug, "debug", false, "Attach an ephemeral debug container to the selected container instead of an exec (distroless images)")
	cmd.PersistentFlags().StringVar(&o.debugImage, "debug-image", iexec.DefaultDebugImage, "Image of the --debug container")
	o.configFlags.AddFlags(cmd.Flags())

	return cmd
}

func (o *IExecOptions) Complete(cmd *cobra.Command, args []string) error {
	var err error

	o.clientCfg, err = o.configFlags.ToRESTConfig()
	if err != nil {
		return errors.Wrap(err, "unable to get rest client")
	}

	// The loader honors --namespace, then the namespace of --context, then the current context.
	o.namespace, _, err = o.configFlags.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return errors.Wrap(err, "unable to get namespace")
	}

	if o.allNamespaces {
		o.namespace = ""
	}

	switch o.lvl {
	case "trace":
		log.SetLevel(log.TraceLevel)
	case "debug":
		log.SetLevel(log.DebugLevel)
	case "info":
		log.SetLevel(log.InfoLevel)
	case "warn":
		log.SetLevel(log.WarnLevel)
	case "error":
		log.SetLevel(log.ErrorLevel)
	case "fatal":
		log.SetLevel(log.FatalLevel)
	case "panic":
		log.SetLevel(log.PanicLevel)
	default:
		log.SetLevel(log.ErrorLevel)
	}

	return nil
}

func (o *IExecOptions) Run(ctx context.Context, args []string) error {
	podFilter := ""

	if len(args) > 0 {
		podFilter = args[0]
	}

	o.remoteCmd = remoteCommand(args)

	config := &iexec.Config{
		Namespace:       o.namespace,
		LabelSelector:   o.labelSelector,
		Naked:           o.naked,
		First:           o.first,
		AllPhases:       o.allPhases,
		Debug:           o.debug,
		DebugImage:      o.debugImage,
		TTY:             iexec.StdioIsTerminal(),
		PodFilter:       podFilter,
		ContainerFilter: o.containerFilter,
		RemoteCmd:       o.remoteCmd,
	}

	// 1) Create a real Kubernetes clientset from o.clientCfg.
	clientset, err := kubernetes.NewForConfig(o.clientCfg)
	if err != nil {
		return errors.Wrap(err, "unable to create kubernetes client")
	}

	// 2) Create your real implementations for the interfaces.
	ui := iexec.NewHuhTerminal()                       // TerminalUI
	k8s := iexec.NewRealK8sClient(clientset)           // K8sClient
	execRunner := iexec.NewStreamExecRunner(clientset) // ExecRunner

	// 3) Pass them to NewIexec along with your config.
	r := iexec.NewIexec(o.clientCfg, config, ui, k8s, execRunner)

	// 4) Run!
	return r.Do(ctx)
}
