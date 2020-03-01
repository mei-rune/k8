package js

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/dop251/goja"
	"github.com/pkg/errors"
	"github.com/runner-mei/k8/js/common"
	"github.com/runner-mei/k8/js/compiler"
	jslib "github.com/runner-mei/k8/js/lib"
	"github.com/runner-mei/k8/lib"
	"github.com/runner-mei/log"
	"golang.org/x/tools/godoc/vfs"
)

//nolint:gochecknoglobals
var (
	errInterrupt     = errors.New("context cancelled")
	ErrMethodMissing = errors.New("method is missing")
)

type Program struct {
	Filename string
	// Source   string
	Program *goja.Program
}

// A Builder is a self-contained bundle of scripts and resources.
type Builder struct {
	logger            log.Logger
	dir               string
	env               map[string]string
	filesystems       vfs.NameSpace 
	compiler          *compiler.Compiler
	compatibilityMode compiler.CompatibilityMode
	programs          []Program
	//BaseInitContext *InitContext
}

// NewBuilder creates a new bundle from a source file and a filesystem.
func NewBuilder(logger log.Logger, dir string, filesystems vfs.NameSpace , compatibilityMode string, env map[string]string) (*Builder, error) {
	compatMode, err := lib.ValidateCompatibilityMode(compatibilityMode)
	if err != nil {
		return nil, err
	}

	return &Builder{
		logger:            logger,
		dir:               dir,
		env:               env,
		filesystems:       filesystems,
		compiler:          compiler.New(),
		compatibilityMode: compatMode,
	}, nil
}

func (b *Builder) Compile(filename string, data []byte) error {
	// Compile sources, both ES5 and ES6 are supported.
	pgm, _, err := b.compiler.Compile(string(data), filename, "", "", true, b.compatibilityMode)
	if err != nil {
		return err
	}

	b.programs = append(b.programs, Program{
		Filename: filename,
		Program:  pgm,
	})
	return nil
}

func (b *Builder) Build(rt *goja.Runtime) (*Runner, error) {
	if rt == nil {
		// Make a bundle, instantiate it into a throwaway VM to populate caches.
		rt = goja.New()
	}

	rt.SetFieldNameMapper(common.FieldNameMapper{})
	rt.SetRandSource(common.NewRandSource())
	if b.compatibilityMode == compiler.CompatibilityModeExtended {
		if _, err := rt.RunProgram(jslib.GetCoreJS()); err != nil {
			return nil, err
		}
	}

	ctx := context.Background()
	initCtx := NewInitContext(b.logger, rt, b.compiler, 
		b.compatibilityMode, &ctx, b.filesystems)

	methods := map[string]Method{}

	if len(b.programs) == 1 {
		name, meta, method, err := b.createMethod(rt,
			initCtx, b.programs[0].Filename, b.programs[0].Program, true)
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
			meta:   meta,
			method: method,
		}

		// A Runner is a self-contained instance of a Bundle.
		return &Runner{
			Runtime: rt,
			InitContext: initCtx,
			Context: initCtx.ctxPtr,
			Default: method,
			Methods: methods,
		}, nil
	}

	for _, pgm := range b.programs {
		name, meta, method, err := b.createMethod(rt, 
			initCtx, pgm.Filename, pgm.Program, false)
		if err != nil {
			return nil, err
		}
		methods[name] = Method{
			meta:   meta,
			method: method,
		}
	}

	// A Runner is a self-contained instance of a Bundle.
	return &Runner{
		Runtime: rt,
		InitContext: initCtx, 
		Context: initCtx.ctxPtr,
		Methods: methods,
	}, nil
}

func (b *Builder) createMethod(rt *goja.Runtime, initCtx *InitContext, filename string, pgm *goja.Program, isDefault bool) (string, map[string]interface{}, goja.Callable, error) {
	b.instantiateEnv(rt, initCtx)

	unbindInit := common.BindToGlobal(rt, common.Bind(rt, initCtx, 
		&common.BridgeContext{
		CurrentDir: common.CurrentDir(filepath.Dir(filename)),
		CtxPtr: initCtx.ctxPtr,
	}))
	if _, err := rt.RunProgram(pgm); err != nil {
		return "", nil, nil, err
	}
	unbindInit()

	// Grab exports.
	exportsV := rt.Get("exports")
	if goja.IsNull(exportsV) || goja.IsUndefined(exportsV) {
		return "", nil,nil, errors.New("exports must be an object")
	}
	exports := exportsV.ToObject(rt)

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
	meta, ok := metaV.ToObject(rt).Export().(map[string]interface{})
	if !ok || meta == nil {
		return "", nil, nil, errors.New("meta description must be a object")
	}

	name, ok := meta["name"]
	if !ok || name == nil {
		if isDefault {
			return "", meta, method, nil
		}
		return "", nil, nil, errors.New("name is missing in the meta description")
	}

	return fmt.Sprint(name), meta, method, nil
}

// Instantiates the bundle into an existing runtime. Not public because it also messes with a bunch
// of other things, will potentially thrash data and makes a mess in it if the operation fails.
func (b *Builder) instantiateEnv(rt *goja.Runtime, initCtx *InitContext) *goja.Object {
	exports := rt.NewObject()
	rt.Set("exports", exports)
	module := rt.NewObject()
	_ = module.Set("exports", exports)
	rt.Set("module", module)
	rt.Set("__ENV", b.env)
	rt.Set("console", common.Bind(rt, newConsole(b.logger), 
	&common.BridgeContext{		CtxPtr: initCtx.ctxPtr 	}))
	return exports
}
