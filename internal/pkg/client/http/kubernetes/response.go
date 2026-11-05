package kubernetes

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/amirhossein2831/kubernetes-access-manager/internal/domain"
	rbac "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type listPage struct {
	Metadata metav1.ListMeta   `json:"metadata"`
	Items    []json.RawMessage `json:"items"`
}

func decodeResponse(body io.Reader, out any) error {
	decoder := json.NewDecoder(io.LimitReader(body, 32*1024*1024))
	if err := decoder.Decode(out); err != nil {
		return err
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return errors.New("invalid trailing Kubernetes response")
	}
	return nil
}

func retryAfter(hint string) time.Duration {
	if seconds, err := strconv.Atoi(hint); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	if date, err := http.ParseTime(hint); err == nil {
		return time.Until(date)
	}
	return 0
}

func appendRBAC(state *domain.State, kind string, raw json.RawMessage) error {
	switch kind {
	case "roles":
		var r rbac.Role
		if err := json.Unmarshal(raw, &r); err != nil {
			return err
		}
		state.Roles = append(state.Roles, convertRole("Role", r.ObjectMeta, r.Rules))
	case "clusterroles":
		var r rbac.ClusterRole
		if err := json.Unmarshal(raw, &r); err != nil {
			return err
		}
		state.Roles = append(state.Roles, convertRole("ClusterRole", r.ObjectMeta, r.Rules))
	case "rolebindings":
		var b rbac.RoleBinding
		if err := json.Unmarshal(raw, &b); err != nil {
			return err
		}
		state.Bindings = append(state.Bindings, convertBinding("RoleBinding", b.ObjectMeta, b.RoleRef, b.Subjects))
	case "clusterrolebindings":
		var b rbac.ClusterRoleBinding
		if err := json.Unmarshal(raw, &b); err != nil {
			return err
		}
		state.Bindings = append(state.Bindings, convertBinding("ClusterRoleBinding", b.ObjectMeta, b.RoleRef, b.Subjects))
	}
	return nil
}

func namespaceName(raw json.RawMessage) (string, error) {
	var ns struct {
		Metadata metav1.ObjectMeta `json:"metadata"`
	}
	if err := json.Unmarshal(raw, &ns); err != nil {
		return "", err
	}
	return ns.Metadata.Name, nil
}

func appendResources(state *domain.State, list metav1.APIResourceList, seen map[string]bool) {
	group := ""
	if parts := strings.SplitN(list.GroupVersion, "/", 2); len(parts) == 2 {
		group = parts[0]
	}
	for _, resource := range list.APIResources {
		key := group + "/" + resource.Name
		if !seen[key] {
			state.Resources = append(state.Resources, domain.Resource{
				APIGroup:   group,
				Name:       resource.Name,
				Namespaced: resource.Namespaced,
			})
			seen[key] = true
		}
	}
}

func convertRole(kind string, m metav1.ObjectMeta, rules []rbac.PolicyRule) domain.Role {
	r := domain.Role{
		Kind:      kind,
		Name:      m.Name,
		Namespace: m.Namespace,
		UID:       string(m.UID),
	}
	for _, rule := range rules {
		r.Rules = append(r.Rules, domain.Rule{
			APIGroups:       rule.APIGroups,
			Resources:       rule.Resources,
			Verbs:           rule.Verbs,
			ResourceNames:   rule.ResourceNames,
			NonResourceURLs: rule.NonResourceURLs,
		})
	}
	return r
}

func convertBinding(kind string, m metav1.ObjectMeta, ref rbac.RoleRef, subjects []rbac.Subject) domain.Binding {
	b := domain.Binding{
		Kind:      kind,
		Name:      m.Name,
		Namespace: m.Namespace,
		UID:       string(m.UID),
		RoleKind:  ref.Kind,
		RoleName:  ref.Name,
	}
	for _, s := range subjects {
		b.Subjects = append(b.Subjects, domain.Subject{
			Kind:      s.Kind,
			Name:      s.Name,
			Namespace: s.Namespace,
		})
	}
	return b
}
