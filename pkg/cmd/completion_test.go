package cmd

import (
	"context"
	"reflect"
	"testing"

	"github.com/spf13/cobra"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestTargetCompletions(t *testing.T) {
	running := corev1.PodStatus{Phase: corev1.PodRunning}
	cs := fake.NewClientset(
		&corev1.Pod{Name: "web-1", Namespace: "prod", Status: running},
		&corev1.Pod{Name: "db-0", Namespace: "prod", Status: running},
		&corev1.Pod{Name: "web-other-ns", Namespace: "dev", Status: running},
		&appsv1.Deployment{Name: "web", Namespace: "prod"},
		&appsv1.Deployment{Name: "api", Namespace: "prod"},
	)
	ctx := context.Background()

	tests := []struct {
		toComplete string
		want       []string
		directive  cobra.ShellCompDirective
	}{
		{"we", []string{"web-1"}, cobra.ShellCompDirectiveNoFileComp},
		{"d", []string{"deploy/", "ds/", "db-0"}, cobra.ShellCompDirectiveNoFileComp},
		{"st", []string{"sts/"}, cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace},
		{"deploy/", []string{"deploy/api", "deploy/web"}, cobra.ShellCompDirectiveNoFileComp},
		{"deploy/w", []string{"deploy/web"}, cobra.ShellCompDirectiveNoFileComp},
	}
	for _, tt := range tests {
		got, directive := targetCompletions(ctx, cs, "prod", tt.toComplete)
		if !reflect.DeepEqual(got, tt.want) || directive != tt.directive {
			t.Errorf("targetCompletions(%q) = %v, %d; want %v, %d", tt.toComplete, got, directive, tt.want, tt.directive)
		}
	}
}

func TestContainerNames(t *testing.T) {
	pod := func(name string, containers ...string) corev1.Pod {
		p := corev1.Pod{Name: name}
		for _, c := range containers {
			p.Spec.Containers = append(p.Spec.Containers, corev1.Container{Name: c})
		}
		return p
	}
	pods := []corev1.Pod{pod("web-1", "app", "istio-proxy"), pod("web-2", "app", "istio-proxy"), pod("db-0", "postgres")}

	if got := containerNames(pods, "web", ""); !reflect.DeepEqual(got, []string{"app", "istio-proxy"}) {
		t.Errorf("web pods: %v", got)
	}
	always := corev1.ContainerRestartPolicyAlways
	side := corev1.Pod{Name: "web-3"}
	side.Spec.InitContainers = []corev1.Container{{Name: "cloudsql", RestartPolicy: &always}, {Name: "migrate"}}
	if got := containerNames([]corev1.Pod{side}, "web", ""); !reflect.DeepEqual(got, []string{"cloudsql"}) {
		t.Errorf("native sidecar only: %v", got)
	}
	if got := containerNames(pods, "", "p"); !reflect.DeepEqual(got, []string{"postgres"}) {
		t.Errorf("prefix p: %v", got)
	}
}
