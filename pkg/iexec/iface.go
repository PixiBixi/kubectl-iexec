package iexec

import (
	"context"
	"io"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
)

// TerminalUI is responsible for selecting a pod or container from a list.
type TerminalUI interface {
	SelectPod(pods []corev1.Pod, config Config) (corev1.Pod, error)
	SelectContainer(containers []corev1.Container, config Config) (corev1.Container, error)
}

type K8sClient interface {
	FetchAllPods(ctx context.Context, namespace, labelSelector string, runningOnly bool) (*corev1.PodList, error)
	MatchPods(pods *corev1.PodList, config Config) (*corev1.PodList, error)
	MatchContainers(pod corev1.Pod, config Config) ([]corev1.Container, error)
	AddEphemeralContainer(ctx context.Context, pod corev1.Pod, container corev1.EphemeralContainer) error
	WaitEphemeralContainer(ctx context.Context, pod corev1.Pod, name string, untilExit bool) (corev1.ContainerState, error)
	StreamLogs(ctx context.Context, pod corev1.Pod, container string, w io.Writer) error
}

type ExecRunner interface {
	RunExec(ctx context.Context, restConfig *rest.Config, pod corev1.Pod, container corev1.Container, cmd []string) error
	RunAttach(ctx context.Context, restConfig *rest.Config, pod corev1.Pod, container string) error
}
