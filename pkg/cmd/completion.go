package cmd

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/PixiBixi/kubectl-iexec/v2/pkg/iexec"
)

// completionTimeout bounds the API calls made on <TAB>.
const completionTimeout = 5 * time.Second

// workloadPrefixes are offered next to pod names for the first argument.
var workloadPrefixes = []string{"deploy/", "sts/", "ds/", "rs/", "job/"}

// completionClient resolves the client and namespace from the flags parsed so far.
func (o *IExecOptions) completionClient() (kubernetes.Interface, string, error) {
	cfg, err := o.configFlags.ToRESTConfig()
	if err != nil {
		return nil, "", err
	}
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, "", err
	}
	ns, _, err := o.configFlags.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return nil, "", err
	}
	if o.allNamespaces {
		ns = ""
	}
	return client, ns, nil
}

// completeTarget completes the first argument: pod names and workload
// prefixes, or workload names once a prefix is typed (deploy/<TAB>).
func (o *IExecOptions) completeTarget(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveDefault
	}
	client, ns, err := o.completionClient()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	ctx, cancel := context.WithTimeout(context.Background(), completionTimeout)
	defer cancel()
	return targetCompletions(ctx, client, ns, toComplete)
}

func targetCompletions(ctx context.Context, client kubernetes.Interface, ns, toComplete string) ([]string, cobra.ShellCompDirective) {
	if prefix, _, found := strings.Cut(toComplete, "/"); found {
		names, err := workloadNames(ctx, client, ns, prefix)
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}
		var comps []string
		for _, n := range withPrefix(prefix+"/", names) {
			if strings.HasPrefix(n, toComplete) {
				comps = append(comps, n)
			}
		}
		return comps, cobra.ShellCompDirectiveNoFileComp
	}

	var comps []string
	for _, p := range workloadPrefixes {
		if strings.HasPrefix(p, toComplete) {
			comps = append(comps, p)
		}
	}
	pods, err := client.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{FieldSelector: "status.phase=" + string(corev1.PodRunning)})
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	podNames := 0
	for _, p := range pods.Items {
		if strings.HasPrefix(p.Name, toComplete) {
			comps = append(comps, p.Name)
			podNames++
		}
	}
	// Only prefixes left: keep the cursor after the "/" for the workload name.
	if podNames == 0 && len(comps) > 0 {
		return comps, cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace
	}
	return comps, cobra.ShellCompDirectiveNoFileComp
}

// workloadNames lists the workloads of a kind prefix (deploy, sts, ...).
func workloadNames(ctx context.Context, client kubernetes.Interface, ns, prefix string) ([]string, error) {
	var names []string
	opts := metav1.ListOptions{}
	switch strings.ToLower(prefix) {
	case "deploy", "deployment", "deployments":
		l, err := client.AppsV1().Deployments(ns).List(ctx, opts)
		if err != nil {
			return nil, err
		}
		for _, o := range l.Items {
			names = append(names, o.Name)
		}
	case "sts", "statefulset", "statefulsets":
		l, err := client.AppsV1().StatefulSets(ns).List(ctx, opts)
		if err != nil {
			return nil, err
		}
		for _, o := range l.Items {
			names = append(names, o.Name)
		}
	case "ds", "daemonset", "daemonsets":
		l, err := client.AppsV1().DaemonSets(ns).List(ctx, opts)
		if err != nil {
			return nil, err
		}
		for _, o := range l.Items {
			names = append(names, o.Name)
		}
	case "rs", "replicaset", "replicasets":
		l, err := client.AppsV1().ReplicaSets(ns).List(ctx, opts)
		if err != nil {
			return nil, err
		}
		for _, o := range l.Items {
			names = append(names, o.Name)
		}
	case "job", "jobs":
		l, err := client.BatchV1().Jobs(ns).List(ctx, opts)
		if err != nil {
			return nil, err
		}
		for _, o := range l.Items {
			names = append(names, o.Name)
		}
	}
	return names, nil
}

func withPrefix(prefix string, names []string) []string {
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = prefix + n
	}
	return out
}

// completeNamespaces completes --namespace.
func (o *IExecOptions) completeNamespaces(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	client, _, err := o.completionClient()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	ctx, cancel := context.WithTimeout(context.Background(), completionTimeout)
	defer cancel()
	l, err := client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	var names []string
	for _, n := range l.Items {
		if strings.HasPrefix(n.Name, toComplete) {
			names = append(names, n.Name)
		}
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}

// completeContexts completes --context from the kubeconfig, without an API call.
func (o *IExecOptions) completeContexts(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	raw, err := o.configFlags.ToRawKubeConfigLoader().RawConfig()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	var names []string
	for name := range raw.Contexts {
		if strings.HasPrefix(name, toComplete) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names, cobra.ShellCompDirectiveNoFileComp
}

// completeContainers completes --container with the container names of the
// pods whose name starts with the first argument (all pods when it is empty).
func (o *IExecOptions) completeContainers(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	client, ns, err := o.completionClient()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	ctx, cancel := context.WithTimeout(context.Background(), completionTimeout)
	defer cancel()
	pods, err := client.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{FieldSelector: "status.phase=" + string(corev1.PodRunning)})
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	filter := ""
	if len(args) > 0 {
		filter = args[0]
	}
	// deploy/web: match on the workload name, its pods are named web-<hash>.
	if _, name, found := strings.Cut(filter, "/"); found {
		filter = name
	}
	return containerNames(pods.Items, filter, toComplete), cobra.ShellCompDirectiveNoFileComp
}

func containerNames(pods []corev1.Pod, podFilter, toComplete string) []string {
	seen := map[string]bool{}
	var names []string
	for _, p := range pods {
		if !strings.Contains(p.Name, podFilter) {
			continue
		}
		for _, c := range iexec.ExecContainers(p) {
			if !seen[c.Name] && strings.HasPrefix(c.Name, toComplete) {
				seen[c.Name] = true
				names = append(names, c.Name)
			}
		}
	}
	sort.Strings(names)
	return names
}
