package daemon

import "github.com/itzzritik/forged/cli/internal/config"

type RuntimeSpec struct {
	Binary string
	Args   []string
}

type ServiceCredentials struct {
	MasterPassword string
}

func DefaultRuntimeSpec() (RuntimeSpec, error) {
	binary, err := findBinary()
	if err != nil {
		return RuntimeSpec{}, err
	}
	return normalizeRuntimeSpec(RuntimeSpec{
		Binary: binary,
		Args:   []string{"daemon"},
	})
}

func EnsureService(paths config.Paths, creds ServiceCredentials, runtime RuntimeSpec) error {
	runtime, err := normalizeRuntimeSpec(runtime)
	if err != nil {
		return err
	}
	if err := InstallService(paths, creds.MasterPassword, runtime); err != nil {
		return err
	}
	return RestartService()
}

func normalizeRuntimeSpec(runtime RuntimeSpec) (RuntimeSpec, error) {
	if runtime.Binary == "" {
		defaultRuntime, err := DefaultRuntimeSpec()
		if err != nil {
			return RuntimeSpec{}, err
		}
		runtime.Binary = defaultRuntime.Binary
		if len(runtime.Args) == 0 {
			runtime.Args = append([]string(nil), defaultRuntime.Args...)
		}
	}
	if len(runtime.Args) == 0 {
		runtime.Args = []string{"daemon"}
	}
	return runtime, nil
}
