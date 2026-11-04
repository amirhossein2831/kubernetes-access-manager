package domain

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
