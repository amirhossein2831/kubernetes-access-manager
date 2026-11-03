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
