package irc

import "strings"

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

// CheckAdmin returns true if hostmask matches any admin in the list.
// WARNING: Returns true if adminList is empty (legacy behavior - everyone is admin).
func CheckAdmin(hostmask string, adminList []string) bool {
	if len(adminList) == 0 {
		return true
	}
	for _, admin := range adminList {
		if admin == hostmask {
			return true
		}
	}
	return false
}

// CheckValid determines if a message should be processed.
// Returns true if:
// - Bot was addressed directly, OR
// - Addressed mode is disabled (respond to all), OR
// - Message is private (DM)
// AND there's at least one argument.
func CheckValid(isAddressed, addressedMode, isPrivate bool, argCount int) bool {
	return (isAddressed || !addressedMode || isPrivate) && argCount > 0
}

// CheckPrivate returns true if target is not a channel (doesn't start with #).
func CheckPrivate(target string) bool {
	return !strings.HasPrefix(target, "#")
}
