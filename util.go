package main

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// ErrInvalidEmail is returned when an email address contains invalid characters
var ErrInvalidEmail = errors.New("invalid email address")

// emailRegex is an allowlist regex for valid email addresses.
// Only lowercase alphanumeric characters, dots, underscore, and hyphen are allowed.
// The local part can contain dots, underscores, or hyphens between alphanumeric characters.
// The domain is composed of labels that cannot start or end with a hyphen and are separated by
// single dots (no consecutive dots).
var emailRegex = regexp.MustCompile(`^[a-z0-9_-]+(?:[._-][a-z0-9]+)*@(?:[a-z0-9]+(?:-[a-z0-9]+)*\.)+[a-z]{2,}$`)

// sieveRedirectRegex searches "if true { redirect ...; }", regardless of case and line breaks
var sieveRedirectRegex = regexp.MustCompile(`(?mi)^\s*\bif\b\s*\btrue\b\s*\{\s*\bredirect\b\s+[\s\S]*?;\s*\}`)

// parseEmail splits an email address into local_part and domain
func parseEmail(email string) (localPart, domain string, err error) {
	parts := strings.SplitN(email, "@", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("%w: invalid format", ErrInvalidEmail)
	}
	return parts[0], parts[1], nil
}

// validateEmail checks if an email address matches the allowlist regex.
// Only lowercase alphanumeric characters, dots, plus, underscore, and hyphen are allowed.
// This prevents shell injection and path traversal attacks by design.
func validateEmail(email string) error {
	if !emailRegex.MatchString(email) {
		return fmt.Errorf("%w: does not match allowed regex", ErrInvalidEmail)
	}

	// Additional check for path traversal (double dots)
	if strings.Contains(email, "..") {
		return fmt.Errorf("%w: contains path traversal sequence", ErrInvalidEmail)
	}

	return nil
}

// hasGlobalRedirect checks if input script matches sieveRedirectRegex
func hasGlobalRedirect(sieveContents []byte) bool {
	content := string(sieveContents)

	return sieveRedirectRegex.MatchString(content)
}
