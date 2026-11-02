package iexec

import (
	"context"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ownerKinds maps the accepted spellings (as kubectl takes them) to a kind.
var ownerKinds = map[string]string{
	"deploy": "deployment", "deployment": "deployment", "deployments": "deployment",
	"sts": "statefulset", "statefulset": "statefulset", "statefulsets": "statefulset",
	"ds": "daemonset", "daemonset": "daemonset", "daemonsets": "daemonset",
	"rs": "replicaset", "replicaset": "replicaset", "replicasets": "replicaset",
	"job": "job", "jobs": "job",
}

// ParseOwner splits a "deploy/api" argument into its kind and name. ok is
// false for a plain pod name filter; an unknown kind is an error.
func ParseOwner(arg string) (kind, name string, ok bool, err error) {
	prefix, name, found := strings.Cut(arg, "/")
	if !found {
		return "", "", false, nil
	}
	kind, known := ownerKinds[strings.ToLower(prefix)]
	if !known || name == "" {
		return "", "", false, fmt.Errorf("unsupported workload %q: use deploy/, sts/, ds/, rs/ or job/ followed by a name", arg)
	}
	return kind, name, true, nil
}

// OwnerSelector returns the pod label selector of a workload.
func (r *RealK8sClient) OwnerSelector(ctx context.Context, namespace, kind, name string) (string, error) {
	var sel *metav1.LabelSelector
	var err error
	get := metav1.GetOptions{}
	switch kind {
	case "deployment":
		o, e := r.client.AppsV1().Deployments(namespace).Get(ctx, name, get)
		if e == nil {
			sel = o.Spec.Selector
		}
		err = e
	case "statefulset":
		o, e := r.client.AppsV1().StatefulSets(namespace).Get(ctx, name, get)
		if e == nil {
			sel = o.Spec.Selector
		}
		err = e
	case "daemonset":
		o, e := r.client.AppsV1().DaemonSets(namespace).Get(ctx, name, get)
		if e == nil {
			sel = o.Spec.Selector
		}
		err = e
	case "replicaset":
		o, e := r.client.AppsV1().ReplicaSets(namespace).Get(ctx, name, get)
		if e == nil {
			sel = o.Spec.Selector
		}
		err = e
	case "job":
		o, e := r.client.BatchV1().Jobs(namespace).Get(ctx, name, get)
		if e == nil {
			sel = o.Spec.Selector
		}
		err = e
	default:
		return "", fmt.Errorf("unsupported workload kind %q", kind)
	}
	if err != nil {
		return "", err
	}

	selector, err := metav1.LabelSelectorAsSelector(sel)
	if err != nil {
		return "", errors.Wrapf(err, "invalid selector on %s/%s", kind, name)
	}
	// An empty selector would match every pod of the namespace.
	if selector.Empty() {
		return "", fmt.Errorf("%s/%s has no pod selector", kind, name)
	}
	return selector.String(), nil
}

// mergeSelectors ANDs two label selectors, either of which may be empty.
func mergeSelectors(a, b string) string {
	switch {
	case a == "":
		return b
	case b == "":
		return a
	}
	return a + "," + b
}
