package domain

type User struct {
	Username string   `json:"username"`
	Groups   []string `json:"groups"`
}

type Permission struct {
	APIGroup string   `json:"api_group"`
	Resource string   `json:"resource"`
	Verbs    []string `json:"verbs"`
}

type Policy struct {
	Namespaces  []string     `json:"namespaces"`
	Permissions []Permission `json:"permissions"`
}

type Rule struct {
	APIGroups       []string `json:"api_groups"`
	Resources       []string `json:"resources"`
	Verbs           []string `json:"verbs"`
	ResourceNames   []string `json:"resource_names"`
	NonResourceURLs []string `json:"non_resource_urls,omitempty"`
}
type Role struct {
	Kind, Namespace, Name, UID string
	Rules                      []Rule
}
type Subject struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
}
type Binding struct {
	Kind, Namespace, Name, UID, RoleKind, RoleName string
	Subjects                                       []Subject
}
type Resource struct {
	APIGroup   string
	Name       string
	Namespaced bool
}
type State struct {
	Roles      []Role
	Bindings   []Binding
	Resources  []Resource
	Namespaces []string
}
type Reference struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
	UID       string `json:"uid,omitempty"`
}
type Source struct {
	Binding         Reference `json:"binding"`
	Role            Reference `json:"role"`
	MatchedSubjects []Subject `json:"matched_subjects"`
	Rule            Rule      `json:"rule"`
}
type Grant struct {
	Scope         string   `json:"scope"`
	Namespace     string   `json:"namespace,omitempty"`
	APIGroup      string   `json:"api_group"`
	Resource      string   `json:"resource"`
	Verbs         []string `json:"verbs"`
	ResourceNames []string `json:"resource_names"`
	Sources       []Source `json:"sources"`
}
type NonResourceGrant struct {
	URLs    []string `json:"urls"`
	Verbs   []string `json:"verbs"`
	Sources []Source `json:"sources"`
}
type UserAccess struct {
	User
	Grants            []Grant            `json:"grants"`
	NonResourceGrants []NonResourceGrant `json:"non_resource_grants"`
}
type PermissionKey struct {
	Resource, Verb string
}
type NamespaceKey struct {
	Namespace, Resource, Verb string
}
type Counts struct {
	Namespace map[NamespaceKey]int
	Cluster   map[PermissionKey]int
}
