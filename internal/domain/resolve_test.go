package domain

import (
	"encoding/json"
	"testing"
)

func fixture() State {
	return State{
		Roles: []Role{{
			Kind: "ClusterRole",
			Name: "reader",
			Rules: []Rule{{
				APIGroups: []string{""},
				Resources: []string{"pods"},
				Verbs:     []string{"get"},
			}},
		}},
		Bindings: []Binding{{
			Kind:      "RoleBinding",
			Name:      "dev",
			Namespace: "dev",
			RoleKind:  "ClusterRole",
			RoleName:  "reader",
			Subjects: []Subject{{
				Kind: "Group",
				Name: "developers",
			}, {
				Kind: "User",
				Name: "ali",
			}},
		}},
		Resources: []Resource{{
			Name:       "pods",
			Namespaced: true,
		}, {
			Name:       "secrets",
			Namespaced: true,
		}, {
			Name:       "nodes",
			Namespaced: false,
		}},
	}
}

func TestResolutionPreservesScopeSourcesAndEmptyUsers(t *testing.T) {
	s := fixture()
	s.Bindings = append(s.Bindings, s.Bindings[0])
	s.Bindings[1].Name = "second"
	a, err := Resolve([]User{{
		Username: "ali",
		Groups:   []string{"developers"},
	}, {
		Username: "sara",
	}}, s)
	if err != nil {
		t.Fatal(err)
	}
	if len(a) != 2 || len(a[0].Grants) != 1 || len(a[0].Grants[0].Sources) != 2 || a[0].Grants[0].Scope != "namespace" || a[0].Grants[0].Namespace != "dev" {
		t.Fatalf("bad resolution: %+v", a)
	}
	if a[1].Grants == nil || len(a[1].Grants) != 0 {
		t.Fatal("empty user omitted")
	}
	if len(a[0].Grants[0].Sources[0].MatchedSubjects) != 2 {
		t.Fatal("lost matched subject provenance")
	}
	b, _ := json.Marshal(a)
	s.Bindings[0], s.Bindings[1] = s.Bindings[1], s.Bindings[0]
	again, _ := Resolve([]User{{
		Username: "sara",
	}, {
		Username: "ali",
		Groups:   []string{"developers"},
	}}, s)
	c, _ := json.Marshal(again)
	if string(b) != string(c) {
		t.Fatal("not deterministic")
	}
}

func TestResolutionMissingRelevantReferenceAndScope(t *testing.T) {
	for _, which := range []string{"role", "scope"} {
		t.Run(which, func(t *testing.T) {
			s := fixture()
			if which == "role" {
				s.Roles = nil
			} else {
				s.Resources = nil
			}
			if _, err := Resolve([]User{{
				Username: "ali",
			}}, s); err == nil {
				t.Fatal("accepted unresolved grants")
			}
			if _, err := Resolve([]User{{
				Username: "unrelated",
			}}, s); err != nil {
				t.Fatal("unrelated broken binding invalidates inventory")
			}
		})
	}
}

func TestWildcardRoleBindingDoesNotGrantClusterResources(t *testing.T) {
	s := fixture()
	s.Roles[0].Rules = []Rule{{
		APIGroups: []string{"*"},
		Resources: []string{"*"},
		Verbs:     []string{"*"},
	}}
	a, err := Resolve([]User{{
		Username: "ali",
	}}, s)
	if err != nil {
		t.Fatal(err)
	}
	p := Policy{
		Namespaces: []string{"dev"},
		Permissions: []Permission{{
			Resource: "pods",
			Verbs:    []string{"get"},
		}, {
			Resource: "nodes",
			Verbs:    []string{"patch"},
		}},
	}
	counts := Count(a, p)
	if counts.Namespace[NamespaceKey{
		Namespace: "dev",
		Resource:  "pods",
		Verb:      "get",
	}] != 1 || counts.Cluster[PermissionKey{
		Resource: "nodes",
		Verb:     "patch",
	}] != 0 {
		t.Fatalf("bad counts %+v", counts)
	}
}

func TestSensitiveCountsRespectCorrelatedRulesAndConstraints(t *testing.T) {
	s := fixture()
	s.Bindings[0].Kind = "ClusterRoleBinding"
	s.Bindings[0].Namespace = ""
	s.Roles[0].Rules = []Rule{
		{
			APIGroups: []string{""},
			Resources: []string{"pods"},
			Verbs:     []string{"get"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"secrets"},
			Verbs:     []string{"delete"},
		},
		{
			APIGroups:     []string{""},
			Resources:     []string{"secrets"},
			Verbs:         []string{"create", "get"},
			ResourceNames: []string{"one"},
		},
	}
	a, err := Resolve([]User{{
		Username: "ali",
	}}, s)
	if err != nil {
		t.Fatal(err)
	}
	p := Policy{
		Namespaces: []string{"dev", "prod"},
		Permissions: []Permission{{
			Resource: "secrets",
			Verbs:    []string{"get", "create", "delete"},
		}},
	}
	c := Count(a, p)
	if c.Cluster[PermissionKey{
		Resource: "secrets",
		Verb:     "get",
	}] != 1 || c.Cluster[PermissionKey{
		Resource: "secrets",
		Verb:     "create",
	}] != 0 {
		t.Fatalf("bad constrained counts %+v", c)
	}
	for _, ns := range p.Namespaces {
		if c.Namespace[NamespaceKey{
			Namespace: ns,
			Resource:  "secrets",
			Verb:      "delete",
		}] != 1 {
			t.Fatal("all namespace grant missing")
		}
	}
	s.Roles[0].Rules = s.Roles[0].Rules[:2]
	a, _ = Resolve([]User{{
		Username: "ali",
	}}, s)
	if Count(a, p).Cluster[PermissionKey{
		Resource: "secrets",
		Verb:     "get",
	}] != 0 {
		t.Fatal("cross-grant false match")
	}
}

func TestImplicitGroupsAndServiceAccountsAreNotInvented(t *testing.T) {
	s := fixture()
	s.Bindings[0].Subjects = []Subject{{
		Kind: "Group",
		Name: "system:authenticated",
	}, {
		Kind: "ServiceAccount",
		Name: "ali",
	}}
	a, err := Resolve([]User{{
		Username: "ali",
	}}, s)
	if err != nil || len(a[0].Grants) != 0 {
		t.Fatal("invented subject membership")
	}
}

func TestSubresourceWildcardAndNonResourceScope(t *testing.T) {
	s := fixture()
	s.Resources = append(s.Resources, Resource{
		Name:       "pods/exec",
		Namespaced: true,
	})
	s.Roles[0].Rules = []Rule{{
		APIGroups: []string{""},
		Resources: []string{"*/exec"},
		Verbs:     []string{"create"},
	}, {
		NonResourceURLs: []string{"/healthz"},
		Verbs:           []string{"get"},
	}}
	a, err := Resolve([]User{{
		Username: "ali",
	}}, s)
	if err != nil {
		t.Fatal(err)
	}
	if len(a[0].NonResourceGrants) != 0 {
		t.Fatal("RoleBinding grants non-resource URL")
	}
	c := Count(a, Policy{
		Namespaces: []string{"dev"},
		Permissions: []Permission{{
			Resource: "pods/exec",
			Verbs:    []string{"create"},
		}},
	})
	if c.Namespace[NamespaceKey{
		Namespace: "dev",
		Resource:  "pods/exec",
		Verb:      "create",
	}] != 1 {
		t.Fatal("subresource wildcard not matched")
	}
}
