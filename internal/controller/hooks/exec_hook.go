package hooks

import recipe "github.com/ramendr/recipe/api/v1alpha1"

type ExecHook struct {
	Hook *recipe.Recipe
}

func (e ExecHook) Execute() (bool, error) {
	return false, nil
}
