package query

import (
	"regexp"
	"strings"
)

var secretField = regexp.MustCompile(`(?i)(?:rcon_password|g_password|sv_privatepassword|ftp_password|smtp_password|monitor_secret_key)`)

// Remove the complete line: cvar dumps vary in quoting and may expose default and current values.
func RedactSecrets(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if secretField.MatchString(line) {
			lines[i] = "[secret masqué]"
		}
	}
	return strings.Join(lines, "\n")
}
func SecretField(key string) bool { return secretField.MatchString(key) }
