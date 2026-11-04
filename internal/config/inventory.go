package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

const maxInputBytes = 8 * 1024 * 1024

func decodeFile(path string, target any) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open startup input: %w", err)
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, maxInputBytes+1))
	if err != nil {
		return err
	}
	if len(b) > maxInputBytes {
		return errors.New("startup input exceeds 8 MiB")
	}
	return decodeJSON(b, target)
}

func decodeJSON(b []byte, target any) error {
	if len(b) > maxInputBytes {
		return errors.New("startup input exceeds 8 MiB")
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	if err := uniqueJSON(dec); err != nil {
		return err
	}
	if _, err := dec.Token(); err != io.EOF {
		return errors.New("trailing JSON data")
	}
	dec = json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(target); err != nil {
		return fmt.Errorf("invalid startup JSON: %w", err)
	}
	return nil
}

func uniqueJSON(d *json.Decoder) error {
	token, err := d.Token()
	if err != nil {
		return err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]bool{}
		for d.More() {
			key, err := d.Token()
			if err != nil {
				return err
			}
			k, ok := key.(string)
			if !ok {
				return errors.New("invalid object key")
			}
			if seen[k] {
				return errors.New("duplicate JSON key")
			}
			seen[k] = true
			if err := uniqueJSON(d); err != nil {
				return err
			}
		}
	case '[':
		for d.More() {
			if err := uniqueJSON(d); err != nil {
				return err
			}
		}
	default:
		return errors.New("invalid JSON delimiter")
	}
	_, err = d.Token()
	return err
}

func LoadInventory(path string) ([]User, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, maxInputBytes+1))
	if err != nil {
		return nil, err
	}
	return parseInventory(b)
}

func parseInventory(b []byte) ([]User, error) {
	var input struct {
		Metadata struct {
			Timestamp string `json:"timestamp"`
			Version   string `json:"version"`
		} `json:"metadata"`
		Data []struct {
			Configs []struct {
				ID   string `json:"id"`
				Data string `json:"data"`
			} `json:"configs"`
			Internals map[string]struct {
				Username  string   `json:"username"`
				Groups    []string `json:"groups"`
				Source    string   `json:"source"`
				FirstName string   `json:"first_name"`
				LastName  string   `json:"last_name"`
			} `json:"internals"`
		} `json:"data"`
	}
	if err := decodeJSON(b, &input); err != nil {
		return nil, err
	}
	users := []User{}
	seen := map[string]bool{}
	for _, block := range input.Data {
		for _, u := range block.Internals {
			if strings.TrimSpace(u.Username) == "" || strings.TrimSpace(u.Username) != u.Username {
				return nil, errors.New("invalid username")
			}
			if seen[u.Username] {
				return nil, errors.New("duplicate username")
			}
			seen[u.Username] = true
			groups := map[string]bool{}
			for _, g := range u.Groups {
				if strings.TrimSpace(g) == "" || strings.TrimSpace(g) != g {
					return nil, errors.New("invalid group")
				}
				groups[g] = true
			}
			normalized := []string{}
			for g := range groups {
				normalized = append(normalized, g)
			}
			sort.Strings(normalized)
			users = append(users, User{
				Username: u.Username,
				Groups:   normalized,
			})
		}
	}
	if len(users) == 0 {
		return nil, errors.New("inventory has no users")
	}
	sort.Slice(users, func(i, j int) bool { return users[i].Username < users[j].Username })
	return users, nil
}

func parsePolicy(b []byte) (Policy, error) {
	var p Policy
	if err := decodeJSON(b, &p); err != nil {
		return p, err
	}
	if len(p.Namespaces) == 0 || len(p.Permissions) == 0 {
		return p, errors.New("sensitivity policy must not be empty")
	}
	keys := map[string]bool{}
	for _, permission := range p.Permissions {
		if permission.Resource == "" || len(permission.Verbs) == 0 {
			return p, errors.New("invalid sensitive permission")
		}
		if permission.APIGroup != "" {
			return p, errors.New("initial metric policy supports core API resources only; labels cannot distinguish API groups")
		}
		if keys[permission.Resource] {
			return p, errors.New("duplicate policy resource")
		}
		keys[permission.Resource] = true
		for _, verb := range permission.Verbs {
			if verb == "" || verb == "*" {
				return p, errors.New("policy verbs must be concrete")
			}
		}
	}
	for _, ns := range p.Namespaces {
		if strings.TrimSpace(ns) == "" {
			return p, errors.New("empty sensitive namespace")
		}
	}
	return p, nil
}
