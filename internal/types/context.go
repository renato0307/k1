package types

import (
	"github.com/renato0307/k1/internal/k8s"
	"github.com/renato0307/k1/internal/ui"
)

// AppContext holds app-wide configuration and dependencies
type AppContext struct {
	Theme     *ui.Theme
	Data      k8s.DataProvider
	Formatter k8s.ResourceFormatter
	Provider  k8s.KubeconfigProvider
}

// NewAppContext creates a new application context
func NewAppContext(
	theme *ui.Theme,
	data k8s.DataProvider,
	formatter k8s.ResourceFormatter,
	provider k8s.KubeconfigProvider,
) *AppContext {
	return &AppContext{
		Theme:     theme,
		Data:      data,
		Formatter: formatter,
		Provider:  provider,
	}
}
