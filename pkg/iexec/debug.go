package iexec

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/wait"
	utilexec "k8s.io/client-go/util/exec"
)

// DefaultDebugImage is the image of the debug container when --debug-image is not set.
const DefaultDebugImage = "busybox:1.37"

// debugStartTimeout bounds the image pull and start of the debug container,
// debugExitTimeout the wait for its exit status once the attach returns.
const (
	debugStartTimeout = 2 * time.Minute
	debugExitTimeout  = 10 * time.Second
)

// newDebugContainer builds the ephemeral container. It shares the process
// namespace of target, so the target filesystem is under /proc/1/root.
func newDebugContainer(target, image string, cmd []string, tty bool) corev1.EphemeralContainer {
	return corev1.EphemeralContainer{
		Name:                     "iexec-debug-" + utilrand.String(5),
		Image:                    image,
		ImagePullPolicy:          corev1.PullIfNotPresent,
		Command:                  cmd,
		Stdin:                    true,
		TTY:                      tty,
		TerminationMessagePolicy: corev1.TerminationMessageReadFile,
		TargetContainerName:      target,
	}
}

// AddEphemeralContainer adds the container with a strategic merge patch, like
// kubectl debug, so a concurrent status update of the pod does not conflict.
func (r *RealK8sClient) AddEphemeralContainer(ctx context.Context, pod corev1.Pod, container corev1.EphemeralContainer) error {
	patch, err := json.Marshal(map[string]any{
		"spec": map[string]any{"ephemeralContainers": []corev1.EphemeralContainer{container}},
	})
	if err != nil {
		return errors.Wrap(err, "unable to build the ephemeral container patch")
	}
	_, err = r.client.CoreV1().Pods(pod.Namespace).Patch(ctx, pod.Name, types.StrategicMergePatchType, patch, metav1.PatchOptions{}, "ephemeralcontainers")
	if apierrors.IsForbidden(err) {
		return errors.Wrap(err, "a debug container needs the patch verb on pods/ephemeralcontainers")
	}
	return err
}

// WaitEphemeralContainer returns the container state once it runs (or, with
// untilExit, once it has exited), and fails fast on an image or create error.
func (r *RealK8sClient) WaitEphemeralContainer(ctx context.Context, pod corev1.Pod, name string, untilExit bool) (corev1.ContainerState, error) {
	var state corev1.ContainerState
	timeout := debugStartTimeout
	if untilExit {
		timeout = debugExitTimeout
	}
	err := wait.PollUntilContextTimeout(ctx, 500*time.Millisecond, timeout, true, func(ctx context.Context) (bool, error) {
		p, err := r.client.CoreV1().Pods(pod.Namespace).Get(ctx, pod.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		for _, s := range p.Status.EphemeralContainerStatuses {
			if s.Name != name {
				continue
			}
			state = s.State
			if s.State.Terminated != nil || (s.State.Running != nil && !untilExit) {
				return true, nil
			}
			if w := s.State.Waiting; w != nil && isFatalWaitReason(w.Reason) {
				return false, fmt.Errorf("debug container %s: %s: %s", name, w.Reason, w.Message)
			}
		}
		return false, nil
	})
	return state, err
}

func isFatalWaitReason(reason string) bool {
	switch reason {
	case "ErrImagePull", "ImagePullBackOff", "InvalidImageName", "CreateContainerError", "CreateContainerConfigError":
		return true
	}
	return false
}

// StreamLogs copies the container logs to w, for a debug command that exited
// before the attach.
func (r *RealK8sClient) StreamLogs(ctx context.Context, pod corev1.Pod, container string, w io.Writer) error {
	rc, err := r.client.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &corev1.PodLogOptions{Container: container}).Stream(ctx)
	if err != nil {
		return err
	}
	defer rc.Close() //nolint:errcheck
	_, err = io.Copy(w, rc)
	return err
}

// debug adds a debug container next to target and attaches to it.
func (r *Iexec) debug(ctx context.Context, pod corev1.Pod, target corev1.Container) error {
	ec := newDebugContainer(target.Name, r.config.DebugImage, r.config.RemoteCmd, r.config.TTY)
	_, _ = fmt.Fprintf(r.stderr, "Adding debug container %s (%s) targeting %s/%s\n", ec.Name, ec.Image, pod.Name, target.Name)

	if err := r.k8sClient.AddEphemeralContainer(ctx, pod, ec); err != nil {
		return errors.Wrap(err, "AddEphemeralContainer failed")
	}
	if _, err := r.k8sClient.WaitEphemeralContainer(ctx, pod, ec.Name, false); err != nil {
		return errors.Wrap(err, "WaitEphemeralContainer failed")
	}

	if r.config.TTY {
		_, _ = fmt.Fprintln(r.stderr, "If you don't see a command prompt, try pressing enter.")
	}
	attachErr := r.exec.RunAttach(ctx, r.restConfig, pod, ec.Name)

	// Attach does not carry the exit status, and a short command can exit
	// before the attach starts: read both from the container status.
	state, err := r.k8sClient.WaitEphemeralContainer(ctx, pod, ec.Name, true)
	t := state.Terminated
	if err != nil || t == nil {
		if attachErr != nil {
			return errors.Wrap(attachErr, "RunAttach failed")
		}
		return nil
	}
	if attachErr != nil {
		if err := r.k8sClient.StreamLogs(ctx, pod, ec.Name, r.stdout); err != nil {
			return errors.Wrap(err, "StreamLogs failed")
		}
	}
	if t.ExitCode != 0 {
		return utilexec.CodeExitError{Err: fmt.Errorf("command terminated with exit code %d", t.ExitCode), Code: int(t.ExitCode)}
	}
	return nil
}
