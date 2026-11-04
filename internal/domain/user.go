package domain

import (
	"errors"
	"sort"
	"strings"

	"github.com/amirhossein2831/kubernetes-access-manager/internal/pkg/util"
)

type User struct {
	Username string   `json:"username"`
	Groups   []string `json:"groups"`
}

type userInput struct {
	Metadata userMetadata `json:"metadata"`
	Data     []userBlock  `json:"data"`
}

type userMetadata struct {
	Timestamp string `json:"timestamp"`
	Version   string `json:"version"`
}

type userBlock struct {
	Configs   []userConfig         `json:"configs"`
	Internals map[string]inputUser `json:"internals"`
}

type userConfig struct {
	ID   string `json:"id"`
	Data string `json:"data"`
}

type inputUser struct {
	Username  string   `json:"username"`
	Groups    []string `json:"groups"`
	Source    string   `json:"source"`
	FirstName string   `json:"first_name"`
	LastName  string   `json:"last_name"`
}

func ParseUsers(inputJSON string) ([]User, error) {
	var input userInput
	if err := util.DecodeJSON([]byte(inputJSON), &input); err != nil {
		return nil, err
	}
	users := []User{}
	seen := map[string]bool{}
	for _, block := range input.Data {
		for _, inputUser := range block.Internals {
			user, err := normalizeUser(inputUser)
			if err != nil {
				return nil, err
			}
			if seen[user.Username] {
				return nil, errors.New("duplicate username")
			}
			seen[user.Username] = true
			users = append(users, user)
		}
	}
	if len(users) == 0 {
		return nil, errors.New("inventory has no users")
	}
	sort.Slice(users, func(i, j int) bool { return users[i].Username < users[j].Username })
	return users, nil
}

func normalizeUser(input inputUser) (User, error) {
	if strings.TrimSpace(input.Username) == "" || strings.TrimSpace(input.Username) != input.Username {
		return User{}, errors.New("invalid username")
	}
	groups, err := normalizeGroups(input.Groups)
	if err != nil {
		return User{}, err
	}
	return User{
		Username: input.Username,
		Groups:   groups,
	}, nil
}

func normalizeGroups(groups []string) ([]string, error) {
	seen := map[string]bool{}
	for _, group := range groups {
		if strings.TrimSpace(group) == "" || strings.TrimSpace(group) != group {
			return nil, errors.New("invalid group")
		}
		seen[group] = true
	}
	normalized := []string{}
	for group := range seen {
		normalized = append(normalized, group)
	}
	sort.Strings(normalized)
	return normalized, nil
}
