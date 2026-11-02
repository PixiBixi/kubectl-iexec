package iexec

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func pod(name string, phase corev1.PodPhase, terminating bool) corev1.Pod {
	p := corev1.Pod{
		Name:   name,
		Status: corev1.PodStatus{Phase: phase},
	}
	if terminating {
		now := metav1.Now()
		p.DeletionTimestamp = &now
	}
	return p
}

func podNames(pods []corev1.Pod) []string {
	names := make([]string, 0, len(pods))
	for _, p := range pods {
		names = append(names, p.Name)
	}
	return names
}

func TestMatchPods_PhaseFilter(t *testing.T) {
	list := &corev1.PodList{Items: []corev1.Pod{
		pod("api-running", corev1.PodRunning, false),
		pod("api-pending", corev1.PodPending, false),
		pod("api-done", corev1.PodSucceeded, false),
		pod("api-terminating", corev1.PodRunning, true),
		pod("worker", corev1.PodRunning, false),
	}}
	c := NewRealK8sClient(nil)

	got, err := c.MatchPods(list, Config{PodFilter: "api"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if names := strings.Join(podNames(got.Items), ","); names != "api-running" {
		t.Errorf("default: got %q, want %q", names, "api-running")
	}

	got, err = c.MatchPods(list, Config{PodFilter: "api", AllPhases: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Items) != 4 {
		t.Errorf("--all-phases: got %v, want the 4 api pods", podNames(got.Items))
	}
}

func TestMatchPods_OnlyNotRunning(t *testing.T) {
	list := &corev1.PodList{Items: []corev1.Pod{
		pod("job-1", corev1.PodSucceeded, false),
		pod("job-2", corev1.PodFailed, false),
	}}

	_, err := NewRealK8sClient(nil).MatchPods(list, Config{PodFilter: "job"})
	if err == nil || !strings.Contains(err.Error(), "--all-phases") {
		t.Errorf("got error %v, want a hint about --all-phases", err)
	}
}

func TestFetchAllPods_FieldSelector(t *testing.T) {
	for _, tt := range []struct {
		runningOnly bool
		want        string
	}{
		{true, "status.phase=Running"},
		{false, ""},
	} {
		cs := fake.NewClientset()
		if _, err := NewRealK8sClient(cs).FetchAllPods("ns", "app=api", tt.runningOnly); err != nil {
			t.Fatalf("FetchAllPods: %v", err)
		}
		action, ok := cs.Actions()[0].(k8stesting.ListAction)
		if !ok {
			t.Fatalf("got action %T, want a ListAction", cs.Actions()[0])
		}
		list := action.GetListRestrictions()
		if got := list.Fields.String(); got != tt.want {
			t.Errorf("runningOnly=%v: field selector %q, want %q", tt.runningOnly, got, tt.want)
		}
		if got := list.Labels.String(); got != "app=api" {
			t.Errorf("label selector %q, want %q", got, "app=api")
		}
	}
}
