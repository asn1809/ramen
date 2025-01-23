package hooks

import "fmt"

// Hook interface will help in executing the hooks based on the types.
// Supported types are "check", "scale" and "exec". The implementor needs
// return the result which would be boolean and error if any.
type HookI interface {
	Execute() (bool, error)
}

// Based on the hook type, return the appropriate implementation of the hook.
func GetHookBasedOnType(hookType string) (HookI, error) {
	switch hookType {
	case "check":
		return CheckHook{}, nil
	case "exec":
		return ExecHook{}, nil
	default:
		return nil, fmt.Errorf("unsupported hook type")
	}
}
