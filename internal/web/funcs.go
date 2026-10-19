package web

import (
	"html/template"
)

// funcMap exposes the small amount of formatting the templates need, so money
// and label handling stay in Go rather than in template expressions.
var funcMap = template.FuncMap{
	"typeLabel":     typeLabel,
	"bucketLabel":   bucketLabel,
	"priorityLabel": priorityLabel,
	"statusLabel":   statusLabel,
	"moneyLabel":    moneyLabel,
	"periodQuery":   periodQuery,
	"pct":           percent,
}

// percent renders a 0..1 fraction as a whole percentage.
func percent(fraction float64) int {
	pct := int(fraction * 100)
	switch {
	case pct < 0:
		return 0
	case pct > 100:
		return 100
	}
	return pct
}
