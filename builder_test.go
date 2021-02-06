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

package k8

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/dop251/goja"
	"github.com/runner-mei/gojs/js/compiler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/godoc/vfs"
	"golang.org/x/tools/godoc/vfs/mapfs"
)

const isWindows = runtime.GOOS == "windows"

func getSimpleBuilder(filename, data string, opts ...interface{}) (*Builder, error) {
	var (
		//fs                = mapfs.New(map[string]string{})
		//env               = map[string]string{}
		compatibilityMode compiler.CompatibilityMode
	)
	for _, o := range opts {
		switch opt := o.(type) {
		// case vfs.FileSystem:
		// 	fs = opt
		// case map[string]string:
		// 	env = opt
		case compiler.CompatibilityMode:
			compatibilityMode = opt
		}
	}

	// ns := vfs.NewNameSpace()
	// ns.Bind("/", fs, "/", vfs.BindAfter)

	builder, err := NewBuilder(compatibilityMode)
	if err != nil {
		return nil, err
	}

	err = builder.Compile(filename, data)
	return builder, err
}

func TestNewBuilder(t *testing.T) {
	ctx := context.Background()
	t.Run("Blank", func(t *testing.T) {
		b, err := getSimpleBuilder("/script.js", "")
		if err == nil {
			_, err = b.Build(ctx, nil)
		}

		assert.EqualError(t, err, "script must export a default function")
	})
	t.Run("Invalid", func(t *testing.T) {
		b, err := getSimpleBuilder("/script.js", "\x00")
		if err == nil {
			_, err = b.Build(ctx, nil)
		}
		assert.NotNil(t, err)
		if err != nil {
			assert.Contains(t, err.Error(), "SyntaxError: /script.js: Unexpected character '\x00' (1:0)\n> 1 | \x00\n")
		}
	})
	t.Run("Error", func(t *testing.T) {
		b, err := getSimpleBuilder("/script.js", `throw new Error("aaaa");`)
		if err == nil {
			_, err = b.Build(ctx, nil)
		}
		assert.EqualError(t, err, "Error: aaaa at /script.js:1:7(3)")
	})
	t.Run("InvalidExports", func(t *testing.T) {
		b, err := getSimpleBuilder("/script.js", `exports = null`)
		if err == nil {
			_, err = b.Build(ctx, nil)
		}
		assert.EqualError(t, err, "exports must be an object")
	})
	t.Run("DefaultUndefined", func(t *testing.T) {
		b, err := getSimpleBuilder("/script.js", `export default undefined;`)
		if err == nil {
			_, err = b.Build(ctx, nil)
		}
		assert.EqualError(t, err, "script must export a default function")
	})
	t.Run("DefaultNull", func(t *testing.T) {
		b, err := getSimpleBuilder("/script.js", `export default null;`)
		if err == nil {
			_, err = b.Build(ctx, nil)
		}
		assert.EqualError(t, err, "script must export a default function")
	})
	t.Run("DefaultWrongType", func(t *testing.T) {
		b, err := getSimpleBuilder("/script.js", `export default 12345;`)
		if err == nil {
			_, err = b.Build(ctx, nil)
		}
		assert.EqualError(t, err, "default export must be a function")
	})
	t.Run("Minimal", func(t *testing.T) {
		b, err := getSimpleBuilder("/script.js", `export default function() {};`)
		if err == nil {
			_, err = b.Build(ctx, nil)
		}
		assert.NoError(t, err)
	})
	// t.Run("stdin", func(t *testing.T) {
	// 	b, err := getSimpleBuilder("-", `export default function() {};`)
	// 	if assert.NoError(t, err) {
	// 		assert.Equal(t, "file://-", b.Filename.String())
	// 		assert.Equal(t, "file:///", b.BaseInitContext.pwd.String())
	// 	}
	// })
	t.Run("CompatibilityMode", func(t *testing.T) {
		t.Run("Extended/ok/CoreJS", func(t *testing.T) {
			compatMode := compiler.CompatibilityModeExtended
			b, err := getSimpleBuilder("/script.js",
				`export default function() {}; new Set([1, 2, 3, 2, 1]);`, compatMode)
			if err == nil {
				_, err = b.Build(ctx, nil)
			}
			assert.NoError(t, err)
		})
		t.Run("Base/ok/Minimal", func(t *testing.T) {
			compatMode := compiler.CompatibilityModeBase
			b, err := getSimpleBuilder("/script.js",
				`module.exports.default = function() {};`, compatMode)
			if err == nil {
				_, err = b.Build(ctx, nil)
			}
			assert.NoError(t, err)
		})
		t.Run("Base/err", func(t *testing.T) {
			testCases := []struct {
				name       string
				compatMode compiler.CompatibilityMode
				code       string
				expErr     string
			}{
				// {"InvalidCompat", "es1", `export default function() {};`,
				// 	`invalid compatibility mode "es1". Use: "extended", "base"`},
				// ES2015 modules are not supported
				{"Modules", compiler.CompatibilityModeBase, `export default function() {};`,
					"/script.js: Line 1:1 Unexpected reserved word"},
				// Arrow functions are not supported
				{"ArrowFuncs", compiler.CompatibilityModeBase,
					`module.exports.default = function() {}; () => {};`,
					"/script.js: Line 1:42 Unexpected token ) (and 1 more errors)"},
				// ES2015 objects polyfilled by core.js are not supported
				{"CoreJS", compiler.CompatibilityModeBase,
					`module.exports.default = function() {}; new Set([1, 2, 3, 2, 1]);`,
					"ReferenceError: Set is not defined at /script.js:1:45(5)"},
			}

			for _, tc := range testCases {
				tc := tc
				t.Run(tc.name, func(t *testing.T) {
					b, err := getSimpleBuilder("/script.js", tc.code, tc.compatMode)
					if err == nil {
						_, err = b.Build(ctx, nil)
					}
					assert.EqualError(t, err, tc.expErr)
				})
			}
		})
	})
}

func TestOpen(t *testing.T) {
	var testCases = [...]struct {
		name           string
		openPath       string
		pwd            string
		isError        bool
		isArchiveError bool
	}{
		{
			name:     "notOpeningUrls",
			openPath: "github.com",
			isError:  true,
			pwd:      "/path/to",
		},
		{
			name:     "simple",
			openPath: "file.txt",
			pwd:      "/path/to",
		},
		{
			name:     "simple with dot",
			openPath: "./file.txt",
			pwd:      "/path/to",
		},
		{
			name:     "simple with two dots",
			openPath: "../to/file.txt",
			pwd:      "/path/not",
		},
		{
			name:     "fullpath",
			openPath: "/path/to/file.txt",
			pwd:      "/path/to",
		},
		{
			name:     "fullpath2",
			openPath: "/path/to/file.txt",
			pwd:      "/path",
		},
		{
			name:     "file is dir",
			openPath: "/path/to/",
			pwd:      "/path/to",
			isError:  true,
		},
		{
			name:     "file is missing",
			openPath: "/path/to/missing.txt",
			isError:  true,
		},
		{
			name:     "relative1",
			openPath: "to/file.txt",
			pwd:      "/path",
		},
		{
			name:     "relative2",
			openPath: "./path/to/file.txt",
			pwd:      "/",
		},
		{
			name:     "relative wonky",
			openPath: "../path/to/file.txt",
			pwd:      "/path",
		},
		{
			name:     "empty open doesn't panic",
			openPath: "",
			pwd:      "/path",
			isError:  true,
		},
	}
	fss := map[string]func() (vfs.FileSystem, string, func()){
		"MemMapFS": func() (vfs.FileSystem, string, func()) {
			return mapfs.New(map[string]string{
				"path/to/file.txt": "hi",
			}), "", func() {}
		},
		"OsFS": func() (vfs.FileSystem, string, func()) {
			prefix, err := ioutil.TempDir("", "k6_open_test")
			require.NoError(t, err)
			filePath := filepath.Join(prefix, "path/to/file.txt")
			require.NoError(t, os.MkdirAll(filepath.Join(prefix, "path/to"), 0755))
			require.NoError(t, ioutil.WriteFile(filePath, []byte(`hi`), 0644))
			return vfs.OS(prefix), prefix, func() { require.NoError(t, os.RemoveAll(prefix)) }
		},
	}

	ctx := context.Background()
	for name, fsInit := range fss {
		t.Run(name, func(t *testing.T) {
			fs, _, cleanUp := fsInit()
			defer cleanUp()

			for _, tCase := range testCases {
				tCase := tCase

				var testFunc = func(t *testing.T) {
					var openPath = tCase.openPath

					var pwd = tCase.pwd
					if pwd == "" {
						pwd = "/path/to/"
					}
					if isWindows {
						openPath = strings.Replace(openPath, `\`, `\\`, -1)
					}
					data := `
						export let file = open("` + openPath + `");
						export default function() { return file };`

					sourceBuilder, err := getSimpleBuilder(filepath.ToSlash(filepath.Join(pwd, "script.js")), data, fs)

					require.NoError(t, err)

					r, err := sourceBuilder.Build(ctx, nil)
					if tCase.isError {
						assert.Error(t, err)
						return
					}
					require.NoError(t, err)
					v, err := r.RunDefaultMethod(context.Background(), goja.Undefined())
					require.NoError(t, err)
					assert.Equal(t, "hi", v)
				}

				t.Run(tCase.name, testFunc)
				if isWindows {

					// windowsify the testcase
					tCase.openPath = strings.Replace(tCase.openPath, `/`, `\`, -1)
					tCase.pwd = strings.Replace(tCase.pwd, `/`, `\`, -1)

					t.Run(tCase.name+" with windows slash", testFunc)
				}
			}
		})
	}
}

func TestBuilderInstantiate(t *testing.T) {
	b, err := getSimpleBuilder("/script.js", `
		export let options = {
			vus: 5,
			teardownTimeout: '1s',
		};
		let val = true;
		export default function() { return val; }
	`)
	if !assert.NoError(t, err) {
		return
	}

	ctx := context.Background()
	bi, err := b.Build(ctx, nil)
	if !assert.NoError(t, err) {
		return
	}

	t.Run("Run", func(t *testing.T) {
		v, err := bi.RunDefaultMethod(ctx, goja.Undefined())
		if assert.NoError(t, err) {
			assert.Equal(t, true, v)
		}
	})

	t.Run("SetAndRun", func(t *testing.T) {
		bi.Runtime.Set("val", false)
		v, err := bi.RunDefaultMethod(ctx, goja.Undefined())
		if assert.NoError(t, err) {
			assert.Equal(t, false, v)
		}
	})
}

func TestBuilderEnv(t *testing.T) {
	env := map[string]string{
		"TEST_A": "1",
		"TEST_B": "",
	}
	data := `
		export default function() {
			if (__ENV.TEST_A !== "1") { throw new Error("Invalid TEST_A: " + __ENV.TEST_A); }
			if (__ENV.TEST_B !== "") { throw new Error("Invalid TEST_B: " + __ENV.TEST_B); }
		}
	`
	b, err := getSimpleBuilder("/script.js", data, env)
	if !assert.NoError(t, err) {
		return
	}

	assert.Equal(t, "1", env["TEST_A"])
	assert.Equal(t, "", env["TEST_B"])

	ctx := context.Background()
	bi, err := b.Build(ctx, nil)
	if assert.NoError(t, err) {
		_, err := bi.RunDefaultMethod(context.Background(), goja.Undefined())
		assert.NoError(t, err)
	}
}
