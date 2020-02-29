package main

import (
	"os"
	"strings"

	"github.com/runner-mei/k8/js"
	"github.com/runner-mei/moo"
	"golang.org/x/tools/godoc/vfs"
)

func parseEnvKeyValue(kv string) (string, string) {
	if idx := strings.IndexRune(kv, '='); idx != -1 {
		return kv[:idx], kv[idx+1:]
	}
	return kv, ""
}

func collectEnv() map[string]string {
	env := make(map[string]string)
	for _, kv := range os.Environ() {
		k, v := parseEnvKeyValue(kv)
		env[k] = v
	}
	return env
}

func New(env *moo.Environment, fs *vfs.NameSpace , filenames []string) (*js.Builder, error) {
	logger := env.Logger

	var envvars map[string]string
	if env.Config.BoolWithDefault("K8_INCLUDE_SYSTEM_ENV_VARS", true) {
		envvars = collectEnv()
	}
	if envvars == nil {
		envvars = map[string]string{}
	}
	compatMode := env.Config.StringWithDefault("K8_COMPATIBILITY_MODE", "")
	dir := env.Fs.FromInstallRoot()

	builder, err := js.NewBuilder(logger, dir, *fs, compatMode, envvars)
	if err != nil {
		return nil, err
	}

	for _, filename := range filenames {
		data, err := vfs.ReadFile(*fs, filename)
		if err != nil {
			return nil, err
		}
		err = builder.Compile(filename, data)
		if err != nil {
			return nil, err
		}
	}
	return builder, nil
}
