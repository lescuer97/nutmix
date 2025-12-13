package templates

import (
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

// formatNumber formats a number with thousand separators (periods)
func FormatNumber(n uint64) string {
	p := message.NewPrinter(language.German)
	return p.Sprintf("%.0f", float64(n))
}
