package sanitize

import (
	"regexp"
	"strings"
)

var (
	accountIDPattern = regexp.MustCompile(`\b[0-9]{12}\b`)
	instancePattern  = regexp.MustCompile(`\bi-[0-9a-fA-F]{8,}\b`)
	arnPattern       = regexp.MustCompile(`arn` + `:aws:[^[:space:]]+`)
)

func Message(value string, extraSensitiveValues ...string) string {
	sanitized := value
	for _, sensitive := range extraSensitiveValues {
		sensitive = strings.TrimSpace(sensitive)
		if sensitive == "" {
			continue
		}
		sanitized = strings.ReplaceAll(sanitized, sensitive, "[redacted]")
	}

	sanitized = arnPattern.ReplaceAllString(sanitized, "[redacted-arn]")
	sanitized = accountIDPattern.ReplaceAllString(sanitized, "[redacted-account]")
	sanitized = instancePattern.ReplaceAllString(sanitized, "[redacted-instance]")
	return sanitized
}
