package iexec

import (
	"context"
	"fmt"
	"slices"
	"strings"

	log "github.com/sirupsen/logrus"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type RealK8sClient struct {
	client kubernetes.Interface
}

func NewRealK8sClient(client kubernetes.Interface) *RealK8sClient {
	return &RealK8sClient{client: client}
}

// FetchAllPods lists the pods. runningOnly filters on the phase server-side, so
// completed Job pods are not sent over the wire; deletionTimestamp is not
// field-selectable and stays a client-side check in MatchPods.
func (r *RealK8sClient) FetchAllPods(ctx context.Context, namespace, labelSelector string, runningOnly bool) (*corev1.PodList, error) {
	opts := metav1.ListOptions{LabelSelector: labelSelector}
	if runningOnly {
		opts.FieldSelector = "status.phase=" + string(corev1.PodRunning)
	}
	return r.client.CoreV1().Pods(namespace).List(ctx, opts)
}

func (r *RealK8sClient) MatchPods(pods *corev1.PodList, config Config) (*corev1.PodList, error) {
	var result corev1.PodList

	log.WithFields(log.Fields{
		"SearchFilter": config.PodFilter,
	}).Infof("Get all pods for podFilter...")

	for i, pod := range pods.Items {
		if !strings.Contains(pod.GetName(), config.PodFilter) {
			continue
		}
		if !config.AllPhases && !isExecable(pod) {
			continue
		}
		result.Items = append(result.Items, pod)
		log.WithFields(log.Fields{
			"PodName": pod.GetName(),
			"index":   i,
		}).Infof("Found pod...")
	}

	if len(result.Items) == 0 {
		if !config.AllPhases {
			return &result, fmt.Errorf("no running pods found%s (use --all-phases to include pods that are not running)", scope(config))
		}
		return &result, fmt.Errorf("no pods found%s", scope(config))
	}

	return &result, nil
}

// isExecable reports whether a pod can accept an exec: Running and not terminating.
func isExecable(pod corev1.Pod) bool {
	return pod.Status.Phase == corev1.PodRunning && pod.DeletionTimestamp == nil
}

// ExecContainers lists what can be exec'd into: the app containers, then the
// native sidecars (init containers with restartPolicy Always).
func ExecContainers(pod corev1.Pod) []corev1.Container {
	out := slices.Clone(pod.Spec.Containers)
	for _, c := range pod.Spec.InitContainers {
		if c.RestartPolicy != nil && *c.RestartPolicy == corev1.ContainerRestartPolicyAlways {
			out = append(out, c)
		}
	}
	return out
}

func (r *RealK8sClient) MatchContainers(pod corev1.Pod, config Config) ([]corev1.Container, error) {
	all := ExecContainers(pod)
	if config.ContainerFilter == "" {
		return all, nil
	}
	log.WithFields(log.Fields{
		"SearchFilter": config.ContainerFilter,
	}).Infof("Get all containers for containerFilter...")
	// An exact name wins over substring matches, so --first on "app" never picks "app-proxy".
	var exact, partial []corev1.Container
	for _, container := range all {
		switch {
		case container.Name == config.ContainerFilter:
			exact = append(exact, container)
		case strings.Contains(container.Name, config.ContainerFilter):
			partial = append(partial, container)
		}
	}
	if len(exact) > 0 {
		return exact, nil
	}
	if len(partial) == 0 {
		return nil, fmt.Errorf("no containers found for filter: %s", config.ContainerFilter)
	}
	return partial, nil
}

// scope describes what the pod search was narrowed to, for error messages.
func scope(config Config) string {
	switch {
	case config.OwnerKind != "":
		return " for " + config.OwnerKind + "/" + config.OwnerName
	case config.PodFilter != "":
		return " for filter: " + config.PodFilter
	}
	return ""
}
