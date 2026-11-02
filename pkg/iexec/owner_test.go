package iexec

import (
	"context"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
)

func TestParseOwner(t *testing.T) {
	tests := []struct {
		arg      string
		kind     string
		name     string
		isOwner  bool
		hasError bool
	}{
		{"api", "", "", false, false},
		{"", "", "", false, false},
		{"deploy/api", "deployment", "api", true, false},
		{"Deployment/api", "deployment", "api", true, false},
		{"sts/db", "statefulset", "db", true, false},
		{"ds/agent", "daemonset", "agent", true, false},
		{"jobs/migrate", "job", "migrate", true, false},
		{"svc/api", "", "", false, true},
		{"deploy/", "", "", false, true},
	}
	for _, tt := range tests {
		kind, name, ok, err := ParseOwner(tt.arg)
		if kind != tt.kind || name != tt.name || ok != tt.isOwner || (err != nil) != tt.hasError {
			t.Errorf("ParseOwner(%q) = %q, %q, %v, %v", tt.arg, kind, name, ok, err)
		}
	}
}

func TestOwnerSelector(t *testing.T) {
	cs := fake.NewClientset(
		&appsv1.Deployment{
			Name: "api", Namespace: "prod",
			Spec: appsv1.DeploymentSpec{Selector: &metav1.LabelSelector{
				MatchLabels:      map[string]string{"app": "api"},
				MatchExpressions: []metav1.LabelSelectorRequirement{{Key: "tier", Operator: metav1.LabelSelectorOpIn, Values: []string{"web"}}},
			}},
		},
		&appsv1.StatefulSet{
			Name: "empty", Namespace: "prod",
			Spec: appsv1.StatefulSetSpec{Selector: &metav1.LabelSelector{}},
		},
	)
	c := NewRealK8sClient(cs)

	got, err := c.OwnerSelector(context.Background(), "prod", "deployment", "api")
	if err != nil || got != "app=api,tier in (web)" {
		t.Errorf("deployment selector = %q, %v", got, err)
	}
	if _, err := c.OwnerSelector(context.Background(), "prod", "statefulset", "empty"); err == nil {
		t.Error("an empty selector must be refused, it matches every pod")
	}
	if _, err := c.OwnerSelector(context.Background(), "prod", "deployment", "missing"); err == nil {
		t.Error("a missing deployment must be an error")
	}
}

func TestDo_OwnerSelectorMergedWithLabels(t *testing.T) {
	var gotSelector string
	k8s := &MockK8sClient{
		OwnerSelectorFn: func(ns, kind, name string) (string, error) {
			if ns != "prod" || kind != "deployment" || name != "api" {
				t.Errorf("OwnerSelector(%q, %q, %q)", ns, kind, name)
			}
			return "app=api", nil
		},
		FetchAllPodsFn: func(_, lbl string, _ bool) (*corev1.PodList, error) {
			gotSelector = lbl
			return &corev1.PodList{Items: []corev1.Pod{{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "app"}}}}}}, nil
		},
		MatchPodsFn:       func(p *corev1.PodList, _ Config) (*corev1.PodList, error) { return p, nil },
		MatchContainersFn: func(p corev1.Pod, _ Config) ([]corev1.Container, error) { return p.Spec.Containers, nil },
	}
	exec := &MockExecRunner{RunExecFn: func(*rest.Config, corev1.Pod, corev1.Container, []string) error { return nil }}
	cfg := &Config{Namespace: "prod", LabelSelector: "track=canary", OwnerKind: "deployment", OwnerName: "api", First: true}

	if err := NewIexec(&rest.Config{}, cfg, &MockUI{}, k8s, exec).Do(context.Background()); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if gotSelector != "app=api,track=canary" {
		t.Errorf("List selector = %q, want the owner and -l selectors merged", gotSelector)
	}
}

func TestDo_OwnerNeedsNamespace(t *testing.T) {
	cfg := &Config{OwnerKind: "deployment", OwnerName: "api"}
	err := NewIexec(&rest.Config{}, cfg, &MockUI{}, &MockK8sClient{}, &MockExecRunner{}).Do(context.Background())
	if err == nil || !strings.Contains(err.Error(), "--all-namespaces") {
		t.Errorf("got %v, want a hint about --all-namespaces", err)
	}
}
