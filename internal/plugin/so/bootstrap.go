package so

import (
	"context"
	"fmt"

	"github.com/yannick2025-tech/Salvo/internal/core/expr"
	"github.com/yannick2025-tech/Salvo/internal/store/model"
	"github.com/yannick2025-tech/Salvo/internal/store/repo"
)

// InitFromDB loads all enabled SO plugins from the database and registers
// the __so function into the expression registry. Returns the loader
// containing all successfully loaded plugins.
//
// Plugins that fail to load are logged but do not prevent other plugins
// from loading. Callers should log the returned errors for visibility.
func InitFromDB(ctx context.Context, soRepo repo.SOPluginRepo, reg *expr.FunctionRegistry) (*Loader, error) {
	loader := NewLoader()

	plugins, err := soRepo.List(ctx, repo.Filter{
		Status: model.SOPluginStatusEnabled,
	})
	if err != nil {
		return nil, fmt.Errorf("so: list enabled plugins: %w", err)
	}

	var loadErrs error
	for _, p := range plugins {
		_, err := loader.Load(p.FilePath)
		if err != nil {
			loadErrs = fmt.Errorf("so: load %q (id=%d): %w\n%v", p.Name, p.ID, err, loadErrs)
			continue
		}
	}

	// Register __so even if no plugins loaded — it will return a clear
	// "plugin not found" error when called.
	if reg != nil {
		if err := RegisterSO(reg, loader); err != nil {
			return nil, fmt.Errorf("so: register __so: %w", err)
		}
	}

	return loader, loadErrs
}