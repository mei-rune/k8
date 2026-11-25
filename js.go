package k8

import (
	"context"
	"io/fs"
	"io/ioutil"
	"strings"

	"github.com/boo-admin/boo/engine/echov5functions"
	"github.com/labstack/echo/v5"
	"github.com/runner-mei/gojs"
	"github.com/runner-mei/gojs/lib"
	"github.com/runner-mei/log"
	"tech.hengwei.com.cn/go/moo"
)

func New(env *moo.Environment, fsys fs.FS, filenames []string) (*Builder, error) {
	// logger := env.Logger
	opts := &gojs.RuntimeOptions{
		IncludeSystemEnvVars: strings.ToLower(env.Config.StringWithDefault("K8_INCLUDE_SYSTEM_ENV_VARS", "true")) == "true",
		CompatibilityMode:    env.Config.StringWithDefault("K8_COMPATIBILITY_MODE", ""),
	}

	builder, err := NewBuilder(opts)
	if err != nil {
		return nil, err
	}

	for _, filename := range filenames {
		data, err := fs.ReadFile(fsys, filename)
		if err != nil {
			return nil, err
		}
		err = builder.Compile(filename, string(data))
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
	moo.On(func(*moo.Environment) moo.Option {
		return moo.Invoke(func(env *moo.Environment, fsys fs.FS, infilenames InFiles, httpSrv *moo.HTTPServer) error {
			filenames := make([]string, 0, 64)
			for _, nm := range infilenames.Filenames {
				for _, n := range nm {
					filenames = append(filenames, n)
				}
			}
			b, err := New(env, fsys, filenames)
			if err != nil {
				return err
			}

			var results []map[string]interface{}

			ctx := context.Background()
			pool := make(chan *Runner, 100)
			for i := 0; i < 100; i++ {
				r, err := b.Build(ctx, nil)
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

			state, err := lib.NewState(log.FromSlogger(env.Logger).Named("k8"), lib.Options{})
			if err != nil {
				return err
			}

			httpSrv.Engine().Any("/k8/:name", func(c *echo.Context) error {
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
				result, err := r.RunMethod(lib.WithState(echov5functions.GetContext(c), state), c.Param("name"), args)
				if err != nil {
					return echov5functions.ReturnError(c, err)
				}
				return echov5functions.ReturnQueryResult(c, result)
			})

			httpSrv.Engine().POST("/k8/_/run_script", func(c *echo.Context) error {
				r := <-pool
				defer func() {
					pool <- r
				}()

				data, err := ioutil.ReadAll(c.Request().Body)
				if err != nil {
					return echov5functions.ReturnError(c, err)
				}

				tmpR, err := b.BuildString(ctx, r.Runtime, string(data))
				if err != nil {
					return echov5functions.ReturnError(c, err)
				}

				args := tmpR.Runtime.NewObject()
				for k, v := range c.QueryParams() {
					if len(v) == 1 {
						args.Set(k, v[0])
					} else {
						args.Set(k, v)
					}
				}

				result, err := tmpR.RunDefaultMethod(lib.WithState(echov5functions.GetContext(c), state), args)
				if err != nil {
					return echov5functions.ReturnError(c, err)
				}
				return echov5functions.ReturnQueryResult(c, result)
			})

			httpSrv.Engine().GET("/k8/meta/methods", func(c *echo.Context) error {
				return echov5functions.ReturnQueryResult(c, results)
			})
			return nil
		})
	})
}
