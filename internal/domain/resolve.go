package domain

import (
	"slices"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

func canonical(values []string) []string {
	set := map[string]bool{}
	for _, v := range values {
		set[v] = true
	}
	out := []string{}
	for v := range set {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

func encoded(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func normalized(r Rule) Rule {
	r.APIGroups = canonical(r.APIGroups)
	r.Resources = canonical(r.Resources)
	r.Verbs = canonical(r.Verbs)
	r.ResourceNames = canonical(r.ResourceNames)
	r.NonResourceURLs = canonical(r.NonResourceURLs)
	return r
}

func has(values []string, s string) bool {
	return slices.Contains(values, s)
}

func resourceMatches(pattern, resource string) bool {
	if pattern == "*" || pattern == resource {
		return true
	}
	parts := strings.SplitN(resource, "/", 2)
	return len(parts) == 2 && pattern == "*/"+parts[1]
}

func groupMatches(pattern, group string) bool {
	return pattern == "*" || pattern == group
}

func roleKey(kind, namespace, name string) string {
	return kind + "\x00" + namespace + "\x00" + name
}

func Resolve(users []User, state State) ([]UserAccess, error) {
	roles := map[string]Role{}
	for _, r := range state.Roles {
		roles[roleKey(r.Kind, r.Namespace, r.Name)] = r
	}
	out := make([]UserAccess, 0, len(users))
	for _, user := range users {
		a := UserAccess{
			User: User{
				Username: user.Username,
				Groups:   canonical(user.Groups),
			},
			Grants:            []Grant{},
			NonResourceGrants: []NonResourceGrant{},
		}
		grants := map[string]Grant{}
		urls := map[string]NonResourceGrant{}
		for _, b := range state.Bindings {
			matched := []Subject{}
			for _, s := range b.Subjects {
				if s.Kind == "User" && s.Name == user.Username || s.Kind == "Group" && has(user.Groups, s.Name) {
					matched = append(matched, s)
				}
			}
			if len(matched) == 0 {
				continue
			}
			sort.Slice(matched, func(i, j int) bool { return encoded(matched[i]) < encoded(matched[j]) })
			roleNS := ""
			if b.RoleKind == "Role" {
				if b.Kind != "RoleBinding" {
					return nil, fmt.Errorf("invalid binding role kind")
				}
				roleNS = b.Namespace
			}
			role, ok := roles[roleKey(b.RoleKind, roleNS, b.RoleName)]
			if !ok {
				return nil, fmt.Errorf("unresolved role for monitored subject: %s/%s", b.Kind, b.Name)
			}
			for _, raw := range role.Rules {
				rule := normalized(raw)
				source := Source{
					Binding: Reference{
						Kind:      b.Kind,
						Name:      b.Name,
						Namespace: b.Namespace,
						UID:       b.UID,
					},
					Role: Reference{
						Kind:      role.Kind,
						Name:      role.Name,
						Namespace: role.Namespace,
						UID:       role.UID,
					},
					MatchedSubjects: matched,
					Rule:            rule,
				}
				if b.Kind == "ClusterRoleBinding" && len(rule.NonResourceURLs) > 0 {
					g := NonResourceGrant{
						URLs:  rule.NonResourceURLs,
						Verbs: rule.Verbs,
					}
					key := encoded(g)
					g = urlsOrNew(urls, key, g)
					g.Sources = appendSource(g.Sources, source)
					urls[key] = g
				}
				for _, group := range rule.APIGroups {
					for _, resource := range rule.Resources {
						scopes := map[bool]bool{}
						for _, r := range state.Resources {
							if groupMatches(group, r.APIGroup) && resourceMatches(resource, r.Name) {
								scopes[r.Namespaced] = true
							}
						}
						if len(scopes) == 0 {
							return nil, fmt.Errorf("unresolved resource scope: %s/%s", group, resource)
						}
						for namespaced := range scopes {
							scope := "cluster"
							ns := ""
							if namespaced {
								scope = "all_namespaces"
							}
							if b.Kind == "RoleBinding" {
								if !namespaced {
									continue
								}
								scope = "namespace"
								ns = b.Namespace
							}
							g := Grant{
								Scope:         scope,
								Namespace:     ns,
								APIGroup:      group,
								Resource:      resource,
								Verbs:         rule.Verbs,
								ResourceNames: rule.ResourceNames,
							}
							key := encoded(g)
							if old, ok := grants[key]; ok {
								g = old
							}
							g.Sources = appendSource(g.Sources, source)
							grants[key] = g
						}
					}
				}
			}
		}
		for _, g := range grants {
			sort.Slice(g.Sources, func(i, j int) bool { return encoded(g.Sources[i]) < encoded(g.Sources[j]) })
			a.Grants = append(a.Grants, g)
		}
		for _, g := range urls {
			sort.Slice(g.Sources, func(i, j int) bool { return encoded(g.Sources[i]) < encoded(g.Sources[j]) })
			a.NonResourceGrants = append(a.NonResourceGrants, g)
		}
		sort.Slice(a.Grants, func(i, j int) bool { return encoded(a.Grants[i]) < encoded(a.Grants[j]) })
		sort.Slice(a.NonResourceGrants, func(i, j int) bool { return encoded(a.NonResourceGrants[i]) < encoded(a.NonResourceGrants[j]) })
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Username < out[j].Username })
	return out, nil
}

func urlsOrNew(m map[string]NonResourceGrant, key string, g NonResourceGrant) NonResourceGrant {
	if old, ok := m[key]; ok {
		return old
	}
	return g
}

func appendSource(sources []Source, source Source) []Source {
	key := encoded(source)
	for _, s := range sources {
		if encoded(s) == key {
			return sources
		}
	}
	return append(sources, source)
}
