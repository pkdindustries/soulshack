package irc

import (
	"testing"

	"github.com/alexschlessinger/pollytool/tools"
)

// A native factory alone only makes a tool loadable by name; the model is
// offered registry.All(), so the IRC tools must be registered as instances.
func TestRegisterIRCToolsOffersThemToTheModel(t *testing.T) {
	registry := tools.NewToolRegistry([]tools.Tool{}, tools.WithUnsafeNoSandbox())
	RegisterIRCTools(registry)

	offered := make(map[string]bool)
	for _, tool := range registry.All() {
		offered[tool.GetName()] = true
	}

	for _, name := range []string{
		"irc__op", "irc__kick", "irc__ban", "irc__topic", "irc__action",
		"irc__mode_set", "irc__mode_query", "irc__invite", "irc__names", "irc__whois",
	} {
		if !offered[name] {
			t.Errorf("registry.All() does not offer %s", name)
		}
	}
}
