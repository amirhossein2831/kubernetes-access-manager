package domain

import "strings"

func Count(access []UserAccess, policy Policy) Counts {
	c := Counts{Namespace: map[NamespaceKey]int{}, Cluster: map[PermissionKey]int{}}
	for _, p := range policy.Permissions {
		for _, v := range p.Verbs {
			c.Cluster[PermissionKey{p.Resource, v}] = 0
			for _, ns := range policy.Namespaces {
				if p.Resource != "nodes" && p.Resource != "namespaces" {
					c.Namespace[NamespaceKey{ns, p.Resource, v}] = 0
				}
			}
		}
	}
	for _, user := range access {
		local := map[NamespaceKey]bool{}
		global := map[PermissionKey]bool{}
		for _, g := range user.Grants {
			for _, p := range policy.Permissions {
				if !groupMatches(g.APIGroup, p.APIGroup) || !resourceMatches(g.Resource, p.Resource) {
					continue
				}
				for _, v := range p.Verbs {
					if !has(g.Verbs, v) && !has(g.Verbs, "*") {
						continue
					}
					if len(g.ResourceNames) > 0 && (v == "deletecollection" || v == "create" && !strings.Contains(p.Resource, "/")) {
						continue
					}
					clusterResource := p.Resource == "nodes" || p.Resource == "namespaces"
					if g.Scope == "cluster" && !clusterResource || g.Scope != "cluster" && clusterResource {
						continue
					}
					if g.Scope == "cluster" || g.Scope == "all_namespaces" {
						global[PermissionKey{p.Resource, v}] = true
					}
					if !clusterResource {
						for _, ns := range policy.Namespaces {
							if g.Scope == "all_namespaces" || g.Scope == "namespace" && g.Namespace == ns {
								local[NamespaceKey{ns, p.Resource, v}] = true
							}
						}
					}
				}
			}
		}
		for k := range local {
			c.Namespace[k]++
		}
		for k := range global {
			c.Cluster[k]++
		}
	}
	return c
}
