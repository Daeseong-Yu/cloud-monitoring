package sanitize

import "testing"

func TestMessageRedactsSensitiveValues(t *testing.T) {
	databaseURL := "postgres://user:pass@" + "db.example" + "/cloud_monitor"
	accountID := "123456" + "789012"
	instanceID := "i-" + "1234abcd"
	arn := "arn" + ":aws:iam::" + accountID + ":role/example"

	got := Message(
		"database "+databaseURL+" account "+accountID+" instance "+instanceID+" "+arn,
		databaseURL,
	)

	for _, forbidden := range []string{
		databaseURL,
		accountID,
		instanceID,
		"arn" + ":aws",
	} {
		if contains(got, forbidden) {
			t.Fatalf("message contains %q: %s", forbidden, got)
		}
	}
}

func contains(value string, needle string) bool {
	for i := 0; i+len(needle) <= len(value); i++ {
		if value[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
