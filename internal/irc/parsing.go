package irc

import (
	"errors"
	"net"
	"regexp"
	"strings"
	"unicode"

	"github.com/lrstanley/girc"
)

// FoldNick normalizes an IRC nickname using the server's CASEMAPPING.
func FoldNick(nick, caseMapping string) string {
	switch caseMapping {
	case "ascii":
		return strings.ToLower(nick)
	case "rfc1459-strict", "strict-rfc1459":
		return strings.NewReplacer("[", "{", "]", "}", "\\", "|").Replace(strings.ToLower(nick))
	default:
		return girc.ToRFC1459(nick)
	}
}

// CheckAddressed returns true if message starts with botNick followed by a separator or end of string.
func CheckAddressed(message, botNick string) bool {
	// If botNick is empty, it matches everything (legacy behavior from HasPrefix)
	if botNick == "" {
		return true
	}
	if !strings.HasPrefix(message, botNick) {
		return false
	}
	if len(message) == len(botNick) {
		return true
	}
	// Check that the next character is a separator
	next := message[len(botNick)]
	return next == ' ' || next == ':' || next == ','
}

// CheckAdmin matches full hostmasks against case-insensitive admin masks.
// Only * (zero or more characters) and ? (one character) are wildcards.
// WARNING: Returns true if adminList is empty (legacy behavior - everyone is admin).
func CheckAdmin(hostmask string, adminList []string) bool {
	if len(adminList) == 0 {
		return true
	}
	if ValidateAdminMask(hostmask) != nil {
		return false
	}
	for _, admin := range adminList {
		if ValidateAdminMask(admin) != nil {
			continue
		}
		pattern := strings.NewReplacer(`\*`, `[^!@]*`, `\?`, `[^!@]`).Replace(regexp.QuoteMeta(admin))
		matched, _ := regexp.MatchString("(?i)^"+pattern+"$", hostmask)
		if matched {
			return true
		}
	}
	return false
}

// ValidateAdminMask accepts nick!user@host masks, including wildcard patterns
// and server-provided cloaks. Brackets and other regexp syntax are literals.
func ValidateAdminMask(mask string) error {
	if strings.Count(mask, "!") != 1 || strings.Count(mask, "@") != 1 {
		return errors.New("admin mask must use nick!user@host format")
	}
	nick, rest, _ := strings.Cut(mask, "!")
	user, host, _ := strings.Cut(rest, "@")
	if nick == "" || user == "" || host == "" || strings.Contains(nick, "@") {
		return errors.New("admin mask must have a non-empty nick, user, and host")
	}
	if strings.IndexFunc(mask, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) >= 0 {
		return errors.New("admin mask cannot contain whitespace or control characters")
	}
	return nil
}

// CheckPrivate returns true if target is not a channel (doesn't start with #).
func CheckPrivate(target string) bool {
	return !strings.HasPrefix(target, "#")
}

// RFC 2812 compliant patterns
var (
	// Nick: starts with letter or special, followed by letters, digits, special, or hyphen
	// Special chars: [\]^_`{|}
	nickRegex = regexp.MustCompile(`^[a-zA-Z\[\]\\^\x60_{|}][a-zA-Z0-9\[\]\\^\x60_{|}\-]*$`)

	// User: alphanumeric with optional ~ prefix, allows -_.
	userRegex = regexp.MustCompile(`^~?[a-zA-Z0-9_.\-]+$`)

	// Hostname: DNS labels (letters, digits, hyphens) separated by dots
	hostnameRegex = regexp.MustCompile(`^([a-zA-Z0-9]([a-zA-Z0-9\-]*[a-zA-Z0-9])?\.)*[a-zA-Z0-9]([a-zA-Z0-9\-]*[a-zA-Z0-9])?$`)
)

// ValidateHostmask validates an IRC hostmask in the format nick!user@host per RFC 2812.
func ValidateHostmask(hostmask string) error {
	if hostmask == "" {
		return errors.New("hostmask cannot be empty")
	}

	// Find ! and @ positions
	exclamIdx := strings.Index(hostmask, "!")
	atIdx := strings.Index(hostmask, "@")

	if exclamIdx == -1 {
		return errors.New("hostmask must contain '!' (format: nick!user@host)")
	}
	if atIdx == -1 {
		return errors.New("hostmask must contain '@' (format: nick!user@host)")
	}
	if exclamIdx >= atIdx {
		return errors.New("'!' must come before '@' (format: nick!user@host)")
	}

	nick := hostmask[:exclamIdx]
	user := hostmask[exclamIdx+1 : atIdx]
	host := hostmask[atIdx+1:]

	// Validate nick
	if nick == "" {
		return errors.New("nick cannot be empty")
	}
	if len(nick) > 30 {
		return errors.New("nick too long (max 30 characters)")
	}
	if !nickRegex.MatchString(nick) {
		return errors.New("invalid nick: must start with letter or special char, contain only letters, digits, special chars, or hyphens")
	}

	// Validate user
	if user == "" {
		return errors.New("user cannot be empty")
	}
	if len(user) > 64 {
		return errors.New("user too long (max 64 characters)")
	}
	if !userRegex.MatchString(user) {
		return errors.New("invalid user: must be alphanumeric with optional ~ prefix, allows -_.")
	}

	// Validate host
	if host == "" {
		return errors.New("host cannot be empty")
	}
	if len(host) > 253 {
		return errors.New("host too long (max 253 characters)")
	}

	// Check if it's a valid IP address
	if net.ParseIP(host) != nil {
		return nil
	}

	// Check if it's a valid hostname
	if !hostnameRegex.MatchString(host) {
		return errors.New("invalid host: must be a valid hostname or IP address")
	}

	return nil
}
