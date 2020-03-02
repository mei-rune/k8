package k8

import (
	"os"
	"strings"

	"github.com/runner-mei/k8/js"
	"github.com/runner-mei/loong"
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

func New(env *moo.Environment, fs vfs.NameSpace, filenames []string) (*js.Builder, error) {
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

	builder, err := js.NewBuilder(logger, dir, fs, compatMode, envvars)
	if err != nil {
		return nil, err
	}

	for _, filename := range filenames {
		data, err := vfs.ReadFile(fs, filename)
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

type OutFiles struct {
	moo.Out

	Filenames []string `group:"k8_script_files"`
}

type InFiles struct {
	moo.In

	Filenames [][]string `group:"k8_script_files"`
}

func init() {
	moo.On(func() moo.Option {
		return moo.Invoke(func(env *moo.Environment, fs vfs.NameSpace, infilenames InFiles, httpSrv *moo.HTTPServer) error {
			filenames := make([]string, 0, 64)
			for _, nm := range infilenames.Filenames {
				for _, n := range nm {
					filenames = append(filenames, n)
				}
			}
			b, err := New(env, fs, filenames)
			if err != nil {
				return err
			}

			var results []map[string]interface{}

			pool := make(chan *js.Runner, 100)
			for i := 0; i < 100; i++ {
				r, err := b.Build(nil)
				if err != nil {
					return err
				}

				if i == 0 {
					for _, method := range r.Methods {
						results = append(results, method.Meta)
					}
				}
				pool <- r
			}

			httpSrv.Engine().GET("/k8/:name", func(c *loong.Context) error {
				r := <-pool
				defer func() {
					pool <- r
				}()
				args := r.Runtime.NewObject()
				for k, v := range c.QueryParams() {
					if len(v) == 1 {
						args.Set(k, v[0])
					} else {
						args.Set(k, v)
					}
				}
				result, err := r.RunMethod(c.StdContext, c.Param("name"), args)
				if err != nil {
					return c.ReturnError(err)
				}
				return c.ReturnQueryResult(result)
			})


			httpSrv.Engine().GET("/k8/meta/methods", func(c *loong.Context) error {				
				return c.ReturnQueryResult(results)
			})
			return nil
		})
	})
}
