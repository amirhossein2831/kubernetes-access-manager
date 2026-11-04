package domain

import (
	"strings"
	"testing"

	"github.com/amirhossein2831/kubernetes-access-manager/internal/pkg/util"
)

func TestParseUsersRejectsAmbiguity(t *testing.T) {
	for name, input := range map[string]string{
		"duplicate_key":  `{"data":[{"internals":{"id":{"username":"a","username":"b"}}}]}`,
		"duplicate_user": `{"data":[{"internals":{"one":{"username":"a"},"two":{"username":"a"}}}]}`,
		"blank":          `{"data":[{"internals":{"id":{"username":" "}}}]}`,
		"invalid_group":  `{"data":[{"internals":{"id":{"username":"a","groups":[" dev"]}}}]}`,
		"empty":          `{"data":[]}`,
		"trailing":       `{"data":[]} {}`,
		"unknown":        `{"data":[],"typo":true}`,
		"oversized":      strings.Repeat(" ", util.MaxInputBytes+1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseUsers(input); err == nil {
				t.Fatal("accepted invalid user input")
			}
		})
	}
}

func TestParseUsersNormalizesSuppliedGroups(t *testing.T) {
	users, err := ParseUsers(`{"metadata":{"version":"v1","timestamp":"2025-12-10T15:00:00Z"},"data":[{"configs":[],"internals":{"opaque":{"username":"ali","groups":["ops","dev","dev"]}}}]}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 1 || users[0].Username != "ali" || strings.Join(users[0].Groups, ",") != "dev,ops" {
		t.Fatalf("unexpected users: %+v", users)
	}
}
