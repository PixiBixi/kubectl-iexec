package iexec

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
	utilexec "k8s.io/client-go/util/exec"
)

func debugIexec(k8s *MockK8sClient, exec *MockExecRunner, stdout io.Writer) *Iexec {
	pod := corev1.Pod{Name: "api", Namespace: "prod", Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "app"}}}}
	k8s.FetchAllPodsFn = func(string, string, bool) (*corev1.PodList, error) {
		return &corev1.PodList{Items: []corev1.Pod{pod}}, nil
	}
	k8s.MatchPodsFn = func(p *corev1.PodList, _ Config) (*corev1.PodList, error) { return p, nil }
	k8s.MatchContainersFn = func(p corev1.Pod, _ Config) ([]corev1.Container, error) { return p.Spec.Containers, nil }
	cfg := &Config{First: true, Debug: true, DebugImage: "busybox:1.37", RemoteCmd: []string{"sh"}, TTY: true}
	r := NewIexec(&rest.Config{}, cfg, &MockUI{}, k8s, exec)
	r.stdout, r.stderr = stdout, io.Discard
	return r
}

func TestDebug_AttachesToRunningContainer(t *testing.T) {
	var added corev1.EphemeralContainer
	var attached string
	k8s := &MockK8sClient{
		AddEphemeralContainerFn: func(_ corev1.Pod, c corev1.EphemeralContainer) error { added = c; return nil },
		WaitEphemeralContainerFn: func(_ corev1.Pod, _ string, untilExit bool) (corev1.ContainerState, error) {
			if untilExit {
				return corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 7}}, nil
			}
			return corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}, nil
		},
		StreamLogsFn: func(corev1.Pod, string, io.Writer) error {
			t.Error("no log replay after a successful attach")
			return nil
		},
	}
	exec := &MockExecRunner{
		RunExecFn: func(*rest.Config, corev1.Pod, corev1.Container, []string) error {
			t.Error("debug must not exec")
			return nil
		},
		RunAttachFn: func(_ *rest.Config, _ corev1.Pod, c string) error { attached = c; return nil },
	}

	err := debugIexec(k8s, exec, io.Discard).Do(context.Background())
	var exitErr utilexec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitStatus() != 7 {
		t.Errorf("got %v, want the exit code 7 read from the container status", err)
	}
	if !strings.HasPrefix(added.Name, "iexec-debug-") || added.TargetContainerName != "app" || added.Image != "busybox:1.37" || !added.TTY || !added.Stdin {
		t.Errorf("unexpected debug container: %+v", added)
	}
	if attached != added.Name {
		t.Errorf("attached to %q, want %q", attached, added.Name)
	}
}

func TestDebug_ExitedBeforeAttachReplaysLogs(t *testing.T) {
	k8s := &MockK8sClient{
		AddEphemeralContainerFn: func(corev1.Pod, corev1.EphemeralContainer) error { return nil },
		WaitEphemeralContainerFn: func(_ corev1.Pod, _ string, untilExit bool) (corev1.ContainerState, error) {
			if untilExit {
				return corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 2}}, nil
			}
			return corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}, nil
		},
		StreamLogsFn: func(_ corev1.Pod, _ string, w io.Writer) error { _, err := io.WriteString(w, "PID 1\n"); return err },
	}
	exec := &MockExecRunner{
		RunAttachFn: func(*rest.Config, corev1.Pod, string) error { return errors.New("container not found") },
	}

	var out bytes.Buffer
	err := debugIexec(k8s, exec, &out).Do(context.Background())
	var exitErr utilexec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitStatus() != 2 {
		t.Errorf("got %v, want exit code 2", err)
	}
	if out.String() != "PID 1\n" {
		t.Errorf("stdout = %q, want the replayed logs", out.String())
	}
}

func TestDebug_AttachErrorWhileRunning(t *testing.T) {
	k8s := &MockK8sClient{
		AddEphemeralContainerFn: func(corev1.Pod, corev1.EphemeralContainer) error { return nil },
		WaitEphemeralContainerFn: func(_ corev1.Pod, _ string, untilExit bool) (corev1.ContainerState, error) {
			if untilExit {
				return corev1.ContainerState{}, context.DeadlineExceeded
			}
			return corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}, nil
		},
	}
	exec := &MockExecRunner{
		RunAttachFn: func(*rest.Config, corev1.Pod, string) error { return errors.New("connection reset") },
	}
	if err := debugIexec(k8s, exec, io.Discard).Do(context.Background()); err == nil || !strings.Contains(err.Error(), "connection reset") {
		t.Errorf("got %v, want the attach error", err)
	}
}

func TestDebug_AddFailureStops(t *testing.T) {
	k8s := &MockK8sClient{
		AddEphemeralContainerFn: func(corev1.Pod, corev1.EphemeralContainer) error { return errors.New("forbidden") },
	}
	if err := debugIexec(k8s, &MockExecRunner{}, io.Discard).Do(context.Background()); err == nil || !strings.Contains(err.Error(), "forbidden") {
		t.Errorf("got %v, want the add error", err)
	}
}
