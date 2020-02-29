/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2016 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package js

import (
	"context"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"fmt"
	"time"

	"github.com/dop251/goja"
	"github.com/pkg/errors"
	"github.com/runner-mei/k8/js/common"
	"github.com/runner-mei/k8/js/compiler"
	"github.com/runner-mei/k8/js/modules"
	"github.com/runner-mei/log"
	"golang.org/x/tools/godoc/vfs"
)
	
const fileSchemeCouldntBeLoadedMsg = `The moduleSpecifier "%s" couldn't be found on ` +
		`local disk. Make sure that you've specified the right path to the file.`

type programWithSource struct {
	pgm    *goja.Program
	src    string
	module *goja.Object
}

// InitContext provides APIs for use in the init context.
type InitContext struct {
	logger log.Logger
	// Bound runtime; used to instantiate objects.
	runtime  *goja.Runtime
	compiler *compiler.Compiler

	// Pointer to a context that bridged modules are invoked with.
	ctxPtr *context.Context

	// Filesystem to load files and scripts from with the map key being the scheme
	filesystems vfs.NameSpace

	// Cache of loaded programs and files.
	programs map[string]programWithSource

	compatibilityMode compiler.CompatibilityMode
}

// NewInitContext creates a new initcontext with the provided arguments
func NewInitContext(
	logger log.Logger, rt *goja.Runtime, c *compiler.Compiler,
	compatMode compiler.CompatibilityMode,
	ctxPtr *context.Context, filesystems vfs.NameSpace,
) *InitContext {
	return &InitContext{
		logger:            logger,
		runtime:           rt,
		compiler:          c,
		ctxPtr:            ctxPtr,
		filesystems:       filesystems,
		programs:          make(map[string]programWithSource),
		compatibilityMode: compatMode,
	}
}

func newBoundInitContext(base *InitContext, ctxPtr *context.Context, rt *goja.Runtime) *InitContext {
	// we don't copy the exports as otherwise they will be shared and we don't want this.
	// this means that all the files will be executed again but once again only once per compilation
	// of the main file.
	var programs = make(map[string]programWithSource, len(base.programs))
	for key, program := range base.programs {
		programs[key] = programWithSource{
			src: program.src,
			pgm: program.pgm,
		}
	}
	return &InitContext{
		runtime:           rt,
		ctxPtr:            ctxPtr,
		filesystems:       base.filesystems,
		compiler:          base.compiler,
		programs:          programs,
		compatibilityMode: base.compatibilityMode,
	}
}

// Require is called when a module/file needs to be loaded by a script
func (i *InitContext) Require(bc *common.BridgeContext, arg string) goja.Value {
	switch {
	case arg == "k8", strings.HasPrefix(arg, "k8/"):
		// Builtin modules ("k8" or "k8/...") are handled specially, as they don't exist on the
		// filesystem. This intentionally shadows attempts to name your own modules this.
		v, err := i.requireModule(bc, arg)
		if err != nil {
			common.Throw(i.runtime, err)
		}
		return v
	default:
		// Fall back to loading from the filesystem.
		v, err := i.requireFile(bc, arg)
		if err != nil {
			common.Throw(i.runtime, err)
		}
		return v
	}
}

func (i *InitContext) requireModule(bc *common.BridgeContext, name string) (goja.Value, error) {
	mod, ok := modules.Index[name]
	if !ok {
		return nil, errors.Errorf("unknown builtin module: %s", name)
	}
	return i.runtime.ToValue(common.Bind(i.runtime, mod, bc)), nil
}

func (i *InitContext) requireFile(bc *common.BridgeContext, name string) (goja.Value, error) {
	filename := i.resolveFile(bc, name)
	filename = filepath.ToSlash(filename)



	// First, check if we have a cached program already.
	pgm, ok := i.programs[filename]
	if !ok || pgm.module == nil {
		exports := i.runtime.NewObject()
		pgm.module = i.runtime.NewObject()
		_ = pgm.module.Set("exports", exports)

		if pgm.pgm == nil {
			data, err := i.readFile(filename)
			if err != nil {
				if os.IsNotExist(err) {
				 	return goja.Undefined(), errors.Errorf(fileSchemeCouldntBeLoadedMsg, filename)
				}
				return goja.Undefined(), err
			}
			pgm.src = string(data)

			// Compile the sources; this handles ES5 vs ES6 automatically.
			pgm.pgm, err = i.compileImport(pgm.src, filename)
			if err != nil {
				return goja.Undefined(), err
			}
		}

		i.programs[filename] = pgm

		// Run the program.
		f, err := i.runtime.RunProgram(pgm.pgm)
		if err != nil {
			delete(i.programs, filename)
			return goja.Undefined(), err
		}
		if call, ok := goja.AssertFunction(f); ok {
			if _, err = call(exports, pgm.module, exports); err != nil {
				return nil, err
			}
		}
	}

	return pgm.module.Get("exports"), nil
}

func (i *InitContext) compileImport(src, filename string) (*goja.Program, error) {
	pgm, _, err := i.compiler.Compile(src, filename,
		"(function(module, exports){\n", "\n})\n", true, i.compatibilityMode)
	return pgm, err
}


func (i *InitContext) resolveFile(bc *common.BridgeContext, name string) string {

	fmt.Println("==== resolveFile", name)
	if strings.HasPrefix(name, ".") {
		if strings.HasPrefix(name, "./") || strings.HasPrefix(name, ".\\") ||
		   strings.HasPrefix(name, "../") || strings.HasPrefix(name, "..\\"){


	fmt.Println("==== resolveFile.join", bc.CurrentDir, name, filepath.Clean(filepath.Join(string(bc.CurrentDir), name)))
			return filepath.Clean(filepath.Join(string(bc.CurrentDir), name))
		}
	}
	return name
}

func (i *InitContext) readFile(name string) ([]byte, error) {
	if strings.Contains(name, "://") {
		data, err := fetch(i.logger, name)
		return  data, err
	}
	if filepath.IsAbs(name) {
		data, err := ioutil.ReadFile(name)
		return  data, err
	}

	data, err := vfs.ReadFile(i.filesystems, filepath.ToSlash(name))
	if err == nil {
		return data, nil
	}
	if !os.IsNotExist(err) {
		if err == os.ErrNotExist {
			err = &os.PathError{
				Op: "read",
			    Path: name,
			    Err: err,
			}
		}
	}
	return  nil, err
}

// Open implements open() in the init context and will read and return the contents of a file
func (i *InitContext) Open(bc *common.BridgeContext, filename string, args ...string) (goja.Value, error) {
	if filename == "" {
		return nil, errors.New("open() can't be used with an empty filename")
	}

	filename = i.resolveFile(bc, filename)
	data, err := i.readFile(filename)
	if err != nil {
		return nil, err
	}

	if len(args) > 0 && args[0] == "b" {
		return i.runtime.ToValue(data), nil
	}
	return i.runtime.ToValue(string(data)), nil
}

func fetch(logger log.Logger, u string) ([]byte, error) {
	logger.Debug("Fetching source...", log.String("url", u))
	startTime := time.Now()
	res, err := http.Get(u)
	if err != nil {
		return nil, err
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != 200 {
		switch res.StatusCode {
		case 404:
			return nil, errors.Errorf("not found: %s", u)
		default:
			return nil, errors.Errorf("wrong status code (%d) for: %s", res.StatusCode, u)
		}
	}

	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	logger.Debug("Fetched!", log.String("url", u),
		log.Duration("t", time.Since(startTime)),
		log.Int("len", len(data)))
	return data, nil
}
