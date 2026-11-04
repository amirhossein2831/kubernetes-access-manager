package domain

import "testing"

func TestParsePolicy(t *testing.T) {
	policy, err := ParsePolicy(`{"namespaces":["default"],"permissions":[{"api_group":"","resource":"secrets","verbs":["get"]}]}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(policy.Namespaces) != 1 || len(policy.Permissions) != 1 || policy.Permissions[0].Resource != "secrets" {
		t.Fatalf("unexpected policy: %+v", policy)
	}
	for _, input := range []string{
		`{}`,
		`{"namespaces":["default"],"permissions":[{"resource":"secrets","verbs":["*"]}]}`,
		`{"namespaces":["default"],"permissions":[{"api_group":"apps","resource":"deployments","verbs":["get"]}]}`,
		`{"namespaces":[" "],"permissions":[{"resource":"secrets","verbs":["get"]}]}`,
	} {
		if _, err := ParsePolicy(input); err == nil {
			t.Fatalf("accepted invalid policy: %s", input)
		}
	}
}
