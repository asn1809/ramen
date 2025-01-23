package hooks

import recipe "github.com/ramendr/recipe/api/v1alpha1"

type CheckHook struct {
	Hook *recipe.Hook
}

func (c CheckHook) Execute() (bool, error) {
	return false, nil
}
