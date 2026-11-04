package domain

import (
	"errors"
	"strings"

	"github.com/amirhossein2831/kubernetes-access-manager/internal/pkg/util"
)

type Permission struct {
	APIGroup string   `json:"api_group"`
	Resource string   `json:"resource"`
	Verbs    []string `json:"verbs"`
}

type Policy struct {
	Namespaces  []string     `json:"namespaces"`
	Permissions []Permission `json:"permissions"`
}

func ParsePolicy(inputJSON string) (Policy, error) {
	var policy Policy
	if err := util.DecodeJSON([]byte(inputJSON), &policy); err != nil {
		return policy, err
	}
	return policy, validatePolicy(policy)
}

func validatePolicy(policy Policy) error {
	if len(policy.Namespaces) == 0 || len(policy.Permissions) == 0 {
		return errors.New("sensitivity policy must not be empty")
	}
	resources := map[string]bool{}
	for _, permission := range policy.Permissions {
		if err := validatePermission(permission); err != nil {
			return err
		}
		if resources[permission.Resource] {
			return errors.New("duplicate policy resource")
		}
		resources[permission.Resource] = true
	}
	for _, namespace := range policy.Namespaces {
		if strings.TrimSpace(namespace) == "" {
			return errors.New("empty sensitive namespace")
		}
	}
	return nil
}

func validatePermission(permission Permission) error {
	if permission.Resource == "" || len(permission.Verbs) == 0 {
		return errors.New("invalid sensitive permission")
	}
	if permission.APIGroup != "" {
		return errors.New("initial metric policy supports core API resources only; labels cannot distinguish API groups")
	}
	for _, verb := range permission.Verbs {
		if verb == "" || verb == "*" {
			return errors.New("policy verbs must be concrete")
		}
	}
	return nil
}
