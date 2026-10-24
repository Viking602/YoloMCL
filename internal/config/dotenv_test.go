package config

import (
	"strings"
	"testing"
)

func TestParseDotenvLiteralSelectedValues(t *testing.T) {
	raw := "\ufeffexport KEY = old\r\n" +
		"KEY = '$2a$literal`$(never-run)#hash' # comment\r\n" +
		"export\tCLIENT = abc # trailing comment\r\n" +
		"OPTIONAL = # not configured\r\n" +
		"IGNORED = 'unclosed\r\n"
	values, err := parseDotenv(strings.NewReader(raw), "KEY", "CLIENT", "OPTIONAL")
	if err != nil {
		t.Fatal(err)
	}
	if values["KEY"] != "$2a$literal`$(never-run)#hash" || values["CLIENT"] != "abc" || values["OPTIONAL"] != "" || len(values) != 3 {
		t.Fatal("literal values, selected keys, or comments were not preserved")
	}
}

func TestParseDotenvMalformedValueDoesNotLeak(t *testing.T) {
	for _, raw := range []string{"KEY='private-token", "KEY='private-token' unexpected"} {
		_, err := parseDotenv(strings.NewReader(raw), "KEY")
		if err == nil || strings.Contains(err.Error(), "private-token") {
			t.Fatal("malformed credentials must fail without exposing their value")
		}
	}
}
