package domain

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
