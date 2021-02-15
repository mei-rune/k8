package k8

import (
	"context"
	"fmt"

	"github.com/dop251/goja"
	"github.com/pkg/errors"
	"github.com/runner-mei/gojs"
)

//nolint:gochecknoglobals
var (
	errInterrupt     = errors.New("context cancelled")
	ErrMethodMissing = errors.New("method is missing")
)

// NewBuilder creates a new bundle from a source file and a filesystem.
func NewBuilder(opts *gojs.RuntimeOptions) (*Builder, error) {
	compatMode, err := gojs.ValidateCompatibilityMode(opts.CompatibilityMode)
	if err != nil {
		return nil, err
	}

	return &Builder{
		c:          gojs.NewCompiler(),
		opts:       opts,
		compatMode: compatMode,
	}, nil
}

type Program struct {
	Filename string
	// Source   string
	Program *goja.Program
}

// A Builder is a self-contained bundle of scripts and resources.
type Builder struct {
	programs   []Program
	c          *gojs.Compiler
	opts       *gojs.RuntimeOptions
	compatMode gojs.CompatibilityMode
}

func (b *Builder) Compile(filename string, code string) error {
	// Compile sources, both ES5 and ES6 are supported.
	pgm, _, err := b.c.Compile(code, filename, "", "", true, b.compatMode)
	if err != nil {
		return err
	}

	b.programs = append(b.programs, Program{
		Filename: filename,
		Program:  pgm,
	})
	return nil
}

func (b *Builder) BuildString(ctx context.Context, rt *gojs.Runtime, script string) (*Runner, error) {
	pgm, _, err := b.c.Compile(script, "_default_", "", "", true, b.compatMode)
	if err != nil {
		return nil, err
	}

	return b.build(ctx, rt, []Program{{
		Filename: "_default_",
		Program:  pgm,
	}})
}

func (b *Builder) Build(ctx context.Context, rt *gojs.Runtime) (*Runner, error) {
	return b.build(ctx, rt, b.programs)
}

func (b *Builder) build(ctx context.Context, rt *gojs.Runtime, programs []Program) (*Runner, error) {
	if rt == nil {
		r, err := gojs.NewWith(b.opts)
		if err != nil {
			return nil, err
		}
		rt = r
	}

	methods := map[string]Method{}
	if len(programs) == 1 {
		name, meta, method, err := b.createMethod(ctx, rt, programs[0].Filename,
			programs[0].Program, true)
		if err != nil {
			return nil, err
		}
		if name == "" {
			name = "default"
		}
		if meta == nil {
			meta = map[string]interface{}{
				"name": name,
			}
		}

		methods[name] = Method{
			Meta:   meta,
			Method: method,
		}

		// A Runner is a self-contained instance of a Bundle.
		return &Runner{
			Runtime: rt,
			Default: method,
			Methods: methods,
		}, nil
	}

	for _, pgm := range programs {
		name, meta, method, err := b.createMethod(ctx, rt,
			pgm.Filename, pgm.Program, false)
		if err != nil {
			return nil, err
		}
		methods[name] = Method{
			Meta:   meta,
			Method: method,
		}
	}

	// A Runner is a self-contained instance of a Bundle.
	return &Runner{
		Runtime: rt,
		Methods: methods,
	}, nil
}

func (b *Builder) createMethod(ctx context.Context, rt *gojs.Runtime, filename string, pgm *goja.Program,
	isDefault bool) (string, map[string]interface{}, goja.Callable, error) {
	gojs.InstantiateEnv(rt)

	if _, err := rt.RunProgram(ctx, pgm); err != nil {
		return "", nil, nil, err
	}

	// Grab exports.
	exportsV := rt.Get("exports")
	if goja.IsNull(exportsV) || goja.IsUndefined(exportsV) {
		return "", nil, nil, errors.New("exports must be an object")
	}
	exports := exportsV.ToObject(rt.Runtime)

	bs, err := exports.MarshalJSON()
	if err != nil {
		panic(err)
	}
	fmt.Println(string(bs))

	// Validate the default function.
	def := exports.Get("default")
	if def == nil || goja.IsNull(def) || goja.IsUndefined(def) {
		return "", nil, nil, errors.New("script must export a default function")
	}
	method, ok := goja.AssertFunction(def)
	if !ok {
		return "", nil, nil, errors.New("default export must be a function")
	}

	metaV := exports.Get("meta")
	if metaV == nil || goja.IsNull(metaV) || goja.IsUndefined(metaV) {
		if isDefault {
			return "", nil, method, nil
		}
		return "", nil, nil, errors.New("script must export a meta description")
	}
	meta, ok := metaV.ToObject(rt.Runtime).Export().(map[string]interface{})
	if !ok || meta == nil {
		return "", nil, nil, errors.New("meta description must be a object")
	}

	id, ok := meta["id"]
	if !ok || id == nil {
		if isDefault {
			return "", meta, method, nil
		}
		return "", nil, nil, errors.New("id is missing in the meta description")
	}

	return fmt.Sprint(id), meta, method, nil
}
