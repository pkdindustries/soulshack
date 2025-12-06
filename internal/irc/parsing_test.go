package irc

import "testing"

func TestCheckAddressed(t *testing.T) {
	tests := []struct {
		name    string
		message string
		nick    string
		want    bool
	}{
		{"exact with colon", "bot: hello", "bot", true},
		{"exact with space", "bot hello", "bot", true},
		{"exact with comma", "bot, hello", "bot", true},
		{"nick prefix matches longer word", "botter hello", "bot", false},
		{"nick in middle", "hello bot", "bot", false},
		{"nick at end", "hello bot", "bot", false},
		{"empty message", "", "bot", false},
		{"empty nick", "bot: hello", "", true}, // HasPrefix with empty prefix is always true
		{"case sensitive", "Bot: hello", "bot", false},
		{"just nick", "bot", "bot", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CheckAddressed(tt.message, tt.nick)
			if got != tt.want {
				t.Errorf("CheckAddressed(%q, %q) = %v, want %v", tt.message, tt.nick, got, tt.want)
			}
		})
	}
}

func TestCheckAdmin_EmptyList(t *testing.T) {
	// WARNING: Empty admin list means everyone is admin!
	// This test documents this security-relevant behavior.
	got := CheckAdmin("anyone!user@host.com", []string{})
	if !got {
		t.Error("CheckAdmin with empty list should return true (everyone is admin)")
	}
}

func TestCheckAdmin_ExactMatch(t *testing.T) {
	admins := []string{"admin!user@trusted.host"}

	tests := []struct {
		name     string
		hostmask string
		want     bool
	}{
		{"exact match", "admin!user@trusted.host", true},
		{"different nick", "other!user@trusted.host", false},
		{"different user", "admin!other@trusted.host", false},
		{"different host", "admin!user@other.host", false},
		{"partial match", "admin!user@trusted", false},
		{"empty hostmask", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CheckAdmin(tt.hostmask, admins)
			if got != tt.want {
				t.Errorf("CheckAdmin(%q, admins) = %v, want %v", tt.hostmask, got, tt.want)
			}
		})
	}
}

func TestCheckAdmin_MultipleAdmins(t *testing.T) {
	admins := []string{
		"admin1!user@host1.com",
		"admin2!user@host2.com",
		"admin3!user@host3.com",
	}

	tests := []struct {
		hostmask string
		want     bool
	}{
		{"admin1!user@host1.com", true},
		{"admin2!user@host2.com", true},
		{"admin3!user@host3.com", true},
		{"admin4!user@host4.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.hostmask, func(t *testing.T) {
			got := CheckAdmin(tt.hostmask, admins)
			if got != tt.want {
				t.Errorf("CheckAdmin(%q) = %v, want %v", tt.hostmask, got, tt.want)
			}
		})
	}
}

func TestCheckValid(t *testing.T) {
	// Truth table for all combinations
	tests := []struct {
		name          string
		isAddressed   bool
		addressedMode bool
		isPrivate     bool
		argCount      int
		want          bool
	}{
		// With args (argCount > 0)
		{"addressed, mode on, channel", true, true, false, 1, true},
		{"addressed, mode off, channel", true, false, false, 1, true},
		{"addressed, mode on, private", true, true, true, 1, true},
		{"addressed, mode off, private", true, false, true, 1, true},
		{"not addressed, mode on, channel", false, true, false, 1, false},  // Key: ignored in channel when mode on
		{"not addressed, mode off, channel", false, false, false, 1, true}, // Mode off = respond to all
		{"not addressed, mode on, private", false, true, true, 1, true},    // Private always works
		{"not addressed, mode off, private", false, false, true, 1, true},

		// Without args (argCount == 0) - all should be false
		{"addressed but no args", true, true, false, 0, false},
		{"private but no args", false, true, true, 0, false},
		{"mode off but no args", false, false, false, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CheckValid(tt.isAddressed, tt.addressedMode, tt.isPrivate, tt.argCount)
			if got != tt.want {
				t.Errorf("CheckValid(addressed=%v, mode=%v, private=%v, args=%d) = %v, want %v",
					tt.isAddressed, tt.addressedMode, tt.isPrivate, tt.argCount, got, tt.want)
			}
		})
	}
}

func TestCheckPrivate(t *testing.T) {
	tests := []struct {
		target string
		want   bool
	}{
		{"#channel", false},
		{"#test", false},
		{"##double", false},
		{"nickname", true},
		{"user123", true},
		{"", true}, // Empty is technically not a channel
	}

	for _, tt := range tests {
		t.Run(tt.target, func(t *testing.T) {
			got := CheckPrivate(tt.target)
			if got != tt.want {
				t.Errorf("CheckPrivate(%q) = %v, want %v", tt.target, got, tt.want)
			}
		})
	}
}

func TestValidateHostmask_Valid(t *testing.T) {
	tests := []struct {
		name     string
		hostmask string
	}{
		{"basic", "nick!user@host.com"},
		{"with tilde prefix", "nick!~user@host.com"},
		{"with hyphen in nick", "nick-name!user@host.com"},
		{"with underscore in user", "nick!user_name@host.com"},
		{"with dot in user", "nick!user.name@host.com"},
		{"ipv4 host", "nick!user@192.168.1.1"},
		{"ipv6 host", "nick!user@::1"},
		{"ipv6 full", "nick!user@2001:db8::1"},
		{"subdomain", "nick!user@sub.domain.example.com"},
		{"special nick chars", "[nick]!user@host.com"},
		{"backslash nick", "nick\\name!user@host.com"},
		{"caret nick", "nick^name!user@host.com"},
		{"backtick nick", "`nick!user@host.com"},
		{"pipe nick", "nick|away!user@host.com"},
		{"curly nick", "{nick}!user@host.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateHostmask(tt.hostmask)
			if err != nil {
				t.Errorf("ValidateHostmask(%q) = %v, want nil", tt.hostmask, err)
			}
		})
	}
}

func TestValidateHostmask_Invalid(t *testing.T) {
	tests := []struct {
		name     string
		hostmask string
		wantErr  string
	}{
		{"empty", "", "cannot be empty"},
		{"no exclamation", "nick@host.com", "must contain '!'"},
		{"no at sign", "nick!userhost.com", "must contain '@'"},
		{"wrong order", "nick@user!host.com", "'!' must come before '@'"},
		{"empty nick", "!user@host.com", "nick cannot be empty"},
		{"empty user", "nick!@host.com", "user cannot be empty"},
		{"empty host", "nick!user@", "host cannot be empty"},
		{"nick starts with digit", "1nick!user@host.com", "invalid nick"},
		{"nick starts with hyphen", "-nick!user@host.com", "invalid nick"},
		{"nick with space", "nick name!user@host.com", "invalid nick"},
		{"user with space", "nick!user name@host.com", "invalid user"},
		{"invalid host", "nick!user@host..com", "invalid host"},
		{"host with space", "nick!user@host name.com", "invalid host"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateHostmask(tt.hostmask)
			if err == nil {
				t.Errorf("ValidateHostmask(%q) = nil, want error containing %q", tt.hostmask, tt.wantErr)
				return
			}
			if !contains(err.Error(), tt.wantErr) {
				t.Errorf("ValidateHostmask(%q) error = %q, want error containing %q", tt.hostmask, err.Error(), tt.wantErr)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
