package iexec

import (
	"context"
	"errors"
	"io"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
)

// Mock UI (no terminal usage)
type MockUI struct {
	SelectPodFn       func([]corev1.Pod, Config) (corev1.Pod, error)
	SelectContainerFn func([]corev1.Container, Config) (corev1.Container, error)
}

func (m *MockUI) SelectPod(pods []corev1.Pod, cfg Config) (corev1.Pod, error) {
	return m.SelectPodFn(pods, cfg)
}
func (m *MockUI) SelectContainer(containers []corev1.Container, cfg Config) (corev1.Container, error) {
	return m.SelectContainerFn(containers, cfg)
}

// Mock K8s
type MockK8sClient struct {
	FetchAllPodsFn    func(namespace, labelSelector string, runningOnly bool) (*corev1.PodList, error)
	MatchPodsFn       func(pods *corev1.PodList, config Config) (*corev1.PodList, error)
	MatchContainersFn func(pod corev1.Pod, config Config) ([]corev1.Container, error)

	AddEphemeralContainerFn  func(pod corev1.Pod, c corev1.EphemeralContainer) error
	WaitEphemeralContainerFn func(pod corev1.Pod, name string, untilExit bool) (corev1.ContainerState, error)
	StreamLogsFn             func(pod corev1.Pod, container string, w io.Writer) error
}

func (m *MockK8sClient) FetchAllPods(_ context.Context, ns, lbl string, runningOnly bool) (*corev1.PodList, error) {
	return m.FetchAllPodsFn(ns, lbl, runningOnly)
}
func (m *MockK8sClient) MatchPods(pods *corev1.PodList, cfg Config) (*corev1.PodList, error) {
	return m.MatchPodsFn(pods, cfg)
}
func (m *MockK8sClient) MatchContainers(pod corev1.Pod, cfg Config) ([]corev1.Container, error) {
	return m.MatchContainersFn(pod, cfg)
}
func (m *MockK8sClient) AddEphemeralContainer(_ context.Context, pod corev1.Pod, c corev1.EphemeralContainer) error {
	return m.AddEphemeralContainerFn(pod, c)
}
func (m *MockK8sClient) WaitEphemeralContainer(_ context.Context, pod corev1.Pod, name string, untilExit bool) (corev1.ContainerState, error) {
	return m.WaitEphemeralContainerFn(pod, name, untilExit)
}
func (m *MockK8sClient) StreamLogs(_ context.Context, pod corev1.Pod, container string, w io.Writer) error {
	return m.StreamLogsFn(pod, container, w)
}

// Mock Exec
type MockExecRunner struct {
	RunExecFn   func(*rest.Config, corev1.Pod, corev1.Container, []string) error
	RunAttachFn func(*rest.Config, corev1.Pod, string) error
}

func (m *MockExecRunner) RunExec(_ context.Context, r *rest.Config, p corev1.Pod, c corev1.Container, cmd []string) error {
	return m.RunExecFn(r, p, c, cmd)
}
func (m *MockExecRunner) RunAttach(_ context.Context, r *rest.Config, p corev1.Pod, c string) error {
	return m.RunAttachFn(r, p, c)
}

// Example Test
func TestIexec_Do_Success(t *testing.T) {
	ui := &MockUI{
		SelectPodFn: func(pods []corev1.Pod, cfg Config) (corev1.Pod, error) {
			return pods[0], nil
		},
		SelectContainerFn: func(containers []corev1.Container, cfg Config) (corev1.Container, error) {
			return containers[0], nil
		},
	}
	k8sMock := &MockK8sClient{
		FetchAllPodsFn: func(ns, lbl string, _ bool) (*corev1.PodList, error) {
			return &corev1.PodList{
				Items: []corev1.Pod{{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{Name: "test-container"}},
					},
				}},
			}, nil
		},
		MatchPodsFn: func(pods *corev1.PodList, cfg Config) (*corev1.PodList, error) {
			return pods, nil
		},
		MatchContainersFn: func(pod corev1.Pod, cfg Config) ([]corev1.Container, error) {
			return pod.Spec.Containers, nil
		},
	}
	execMock := &MockExecRunner{
		RunExecFn: func(r *rest.Config, p corev1.Pod, c corev1.Container, cmd []string) error {
			return nil
		},
	}

	cfg := &Config{Namespace: "default"}
	iex := NewIexec(&rest.Config{}, cfg, ui, k8sMock, execMock)

	if err := iex.Do(context.Background()); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

// Example Test - Error scenario
func TestIexec_Do_FetchPodsError(t *testing.T) {
	ui := &MockUI{}
	k8sMock := &MockK8sClient{
		FetchAllPodsFn: func(ns, lbl string, _ bool) (*corev1.PodList, error) {
			return nil, errors.New("error fetching pods")
		},
		MatchPodsFn:       nil,
		MatchContainersFn: nil,
	}
	execMock := &MockExecRunner{}

	cfg := &Config{Namespace: "default"}
	iex := NewIexec(&rest.Config{}, cfg, ui, k8sMock, execMock)

	err := iex.Do(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "fetchAllPods failed: error fetching pods" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestIexec_Do_First_SkipsPrompts(t *testing.T) {
	ui := &MockUI{
		SelectPodFn: func([]corev1.Pod, Config) (corev1.Pod, error) {
			t.Fatal("SelectPod must not be called with First")
			return corev1.Pod{}, nil
		},
		SelectContainerFn: func([]corev1.Container, Config) (corev1.Container, error) {
			t.Fatal("SelectContainer must not be called with First")
			return corev1.Container{}, nil
		},
	}
	k8sMock := &MockK8sClient{
		FetchAllPodsFn: func(ns, lbl string, _ bool) (*corev1.PodList, error) {
			return &corev1.PodList{Items: []corev1.Pod{
				{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "c1"}, {Name: "c2"}}}},
				{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "other"}}}},
			}}, nil
		},
		MatchPodsFn: func(pods *corev1.PodList, cfg Config) (*corev1.PodList, error) {
			return pods, nil
		},
		MatchContainersFn: func(pod corev1.Pod, cfg Config) ([]corev1.Container, error) {
			return pod.Spec.Containers, nil
		},
	}
	var got string
	execMock := &MockExecRunner{
		RunExecFn: func(_ *rest.Config, _ corev1.Pod, c corev1.Container, _ []string) error {
			got = c.Name
			return nil
		},
	}

	iex := NewIexec(&rest.Config{}, &Config{Namespace: "default", First: true}, ui, k8sMock, execMock)
	if err := iex.Do(context.Background()); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got != "c1" {
		t.Errorf("expected first container c1, got %q", got)
	}
}
