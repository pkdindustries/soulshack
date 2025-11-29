package bot

import (
	"fmt"
	"strings"

	"github.com/mazznoer/colorgrad"
)

// GetBanner returns a colorized ASCII art banner
func GetBanner(version string) string {
	banner := `
 ____                    _   ____    _                      _
/ ___|    ___    _   _  | | / ___|  | |__     __ _    ___  | | __
\___ \   / _ \  | | | | | | \___ \  | '_ \   / _' |  / __| | |/ /
 ___) | | (_) | | |_| | | |  ___) | | | | | | (_| | | (__  |   <
|____/   \___/   \__,_| |_| |____/  |_| |_|  \__,_|  \___| |_|\_\
 .  .  .  because  real  people  are  overrated  [v` + version + `]
`
	grad, _ := colorgrad.NewGradient().
		HtmlColors("#1115f0ff", "#fdfdfdff").
		Build()

	lines := strings.Split(banner, "\n")

	// Find max line length for gradient spread
	maxLen := 0
	for _, line := range lines {
		if len(line) > maxLen {
			maxLen = len(line)
		}
	}

	colors := grad.Colors(uint(maxLen))
	var coloredBanner strings.Builder

	for _, line := range lines {
		for i, ch := range line {
			r, g, b, _ := colors[i].RGBA255()
			coloredBanner.WriteString(fmt.Sprintf("\x1b[38;2;%d;%d;%dm%c", r, g, b, ch))
		}
		coloredBanner.WriteString("\x1b[0m\n")
	}

	return coloredBanner.String()
}
