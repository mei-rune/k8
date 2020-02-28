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
	"runtime"
	"testing"

	"github.com/dop251/goja"
	"github.com/runner-mei/k8/js/compiler"
	"github.com/runner-mei/log"
	"github.com/stretchr/testify/assert"
	"golang.org/x/tools/godoc/vfs"
	"golang.org/x/tools/godoc/vfs/mapfs"
)

const isWindows = runtime.GOOS == "windows"

func getSimpleBuilder(filename, data string, opts ...interface{}) (*Builder, error) {
	var (
		fs                = mapfs.New(map[string]string{})
		env               = map[string]string{}
		compatibilityMode string
	)
	for _, o := range opts {
		switch opt := o.(type) {
		case vfs.FileSystem:
			fs = opt
		case map[string]string:
			env = opt
		case string:
			compatibilityMode = opt
		}
	}

	ns := vfs.NewNameSpace()
	ns.Bind("/", fs, "/", vfs.BindAfter)

	builder, err := NewBuilder(log.Empty(), "", ns, compatibilityMode, env)
	if err != nil {
		return nil, err
	}

	builder.Compile(filename, []byte(data))
	return builder, nil
}

func TestNewBuilder(t *testing.T) {
	t.Run("Blank", func(t *testing.T) {
		_, err := getSimpleBuilder("/script.js", "")
		assert.EqualError(t, err, "script must export a default function")
	})
	t.Run("Invalid", func(t *testing.T) {
		_, err := getSimpleBuilder("/script.js", "\x00")
		assert.NotNil(t, err)
		if err != nil {
			assert.Contains(t, err.Error(), "SyntaxError: file:///script.js: Unexpected character '\x00' (1:0)\n> 1 | \x00\n")
		}
	})
	t.Run("Error", func(t *testing.T) {
		_, err := getSimpleBuilder("/script.js", `throw new Error("aaaa");`)
		assert.EqualError(t, err, "Error: aaaa at file:///script.js:1:7(3)")
	})
	t.Run("InvalidExports", func(t *testing.T) {
		_, err := getSimpleBuilder("/script.js", `exports = null`)
		assert.EqualError(t, err, "exports must be an object")
	})
	t.Run("DefaultUndefined", func(t *testing.T) {
		_, err := getSimpleBuilder("/script.js", `export default undefined;`)
		assert.EqualError(t, err, "script must export a default function")
	})
	t.Run("DefaultNull", func(t *testing.T) {
		_, err := getSimpleBuilder("/script.js", `export default null;`)
		assert.EqualError(t, err, "script must export a default function")
	})
	t.Run("DefaultWrongType", func(t *testing.T) {
		_, err := getSimpleBuilder("/script.js", `export default 12345;`)
		assert.EqualError(t, err, "default export must be a function")
	})
	t.Run("Minimal", func(t *testing.T) {
		_, err := getSimpleBuilder("/script.js", `export default function() {};`)
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
			compatMode := compiler.CompatibilityModeExtended.String()
			_, err := getSimpleBuilder("/script.js",
				`export default function() {}; new Set([1, 2, 3, 2, 1]);`, compatMode)
			assert.NoError(t, err)
		})
		t.Run("Base/ok/Minimal", func(t *testing.T) {
			compatMode := compiler.CompatibilityModeBase.String()
			_, err := getSimpleBuilder("/script.js",
				`module.exports.default = function() {};`, compatMode)
			assert.NoError(t, err)
		})
		t.Run("Base/err", func(t *testing.T) {
			testCases := []struct {
				name       string
				compatMode string
				code       string
				expErr     string
			}{
				{"InvalidCompat", "es1", `export default function() {};`,
					`invalid compatibility mode "es1". Use: "extended", "base"`},
				// ES2015 modules are not supported
				{"Modules", "base", `export default function() {};`,
					"file:///script.js: Line 1:1 Unexpected reserved word"},
				// Arrow functions are not supported
				{"ArrowFuncs", "base",
					`module.exports.default = function() {}; () => {};`,
					"file:///script.js: Line 1:42 Unexpected token ) (and 1 more errors)"},
				// ES2015 objects polyfilled by core.js are not supported
				{"CoreJS", "base",
					`module.exports.default = function() {}; new Set([1, 2, 3, 2, 1]);`,
					"ReferenceError: Set is not defined at file:///script.js:1:45(5)"},
			}

			for _, tc := range testCases {
				tc := tc
				t.Run(tc.name, func(t *testing.T) {
					_, err := getSimpleBuilder("/script.js", tc.code, tc.compatMode)
					assert.EqualError(t, err, tc.expErr)
				})
			}
		})
	})
}

// func TestOpen(t *testing.T) {
// 	var testCases = [...]struct {
// 		name           string
// 		openPath       string
// 		pwd            string
// 		isError        bool
// 		isArchiveError bool
// 	}{
// 		{
// 			name:     "notOpeningUrls",
// 			openPath: "github.com",
// 			isError:  true,
// 			pwd:      "/path/to",
// 		},
// 		{
// 			name:     "simple",
// 			openPath: "file.txt",
// 			pwd:      "/path/to",
// 		},
// 		{
// 			name:     "simple with dot",
// 			openPath: "./file.txt",
// 			pwd:      "/path/to",
// 		},
// 		{
// 			name:     "simple with two dots",
// 			openPath: "../to/file.txt",
// 			pwd:      "/path/not",
// 		},
// 		{
// 			name:     "fullpath",
// 			openPath: "/path/to/file.txt",
// 			pwd:      "/path/to",
// 		},
// 		{
// 			name:     "fullpath2",
// 			openPath: "/path/to/file.txt",
// 			pwd:      "/path",
// 		},
// 		{
// 			name:     "file is dir",
// 			openPath: "/path/to/",
// 			pwd:      "/path/to",
// 			isError:  true,
// 		},
// 		{
// 			name:     "file is missing",
// 			openPath: "/path/to/missing.txt",
// 			isError:  true,
// 		},
// 		{
// 			name:     "relative1",
// 			openPath: "to/file.txt",
// 			pwd:      "/path",
// 		},
// 		{
// 			name:     "relative2",
// 			openPath: "./path/to/file.txt",
// 			pwd:      "/",
// 		},
// 		{
// 			name:     "relative wonky",
// 			openPath: "../path/to/file.txt",
// 			pwd:      "/path",
// 		},
// 		{
// 			name:     "empty open doesn't panic",
// 			openPath: "",
// 			pwd:      "/path",
// 			isError:  true,
// 		},
// 	}
// 	fss := map[string]func() (afero.Fs, string, func()){
// 		"MemMapFS": func() (afero.Fs, string, func()) {
// 			fs := afero.NewMemMapFs()
// 			require.NoError(t, fs.MkdirAll("/path/to", 0755))
// 			require.NoError(t, afero.WriteFile(fs, "/path/to/file.txt", []byte(`hi`), 0644))
// 			return fs, "", func() {}
// 		},
// 		"OsFS": func() (afero.Fs, string, func()) {
// 			prefix, err := ioutil.TempDir("", "k6_open_test")
// 			require.NoError(t, err)
// 			fs := afero.NewOsFs()
// 			filePath := filepath.Join(prefix, "/path/to/file.txt")
// 			require.NoError(t, fs.MkdirAll(filepath.Join(prefix, "/path/to"), 0755))
// 			require.NoError(t, afero.WriteFile(fs, filePath, []byte(`hi`), 0644))
// 			if isWindows {
// 				fs = fsext.NewTrimFilePathSeparatorFs(fs)
// 			}
// 			return fs, prefix, func() { require.NoError(t, os.RemoveAll(prefix)) }
// 		},
// 	}

// 	for name, fsInit := range fss {
// 		fs, prefix, cleanUp := fsInit()
// 		defer cleanUp()
// 		fs = afero.NewReadOnlyFs(fs)
// 		t.Run(name, func(t *testing.T) {
// 			for _, tCase := range testCases {
// 				tCase := tCase

// 				var testFunc = func(t *testing.T) {
// 					var openPath = tCase.openPath
// 					// if fullpath prepend prefix
// 					if openPath != "" && (openPath[0] == '/' || openPath[0] == '\\') {
// 						openPath = filepath.Join(prefix, openPath)
// 					}
// 					if isWindows {
// 						openPath = strings.Replace(openPath, `\`, `\\`, -1)
// 					}
// 					var pwd = tCase.pwd
// 					if pwd == "" {
// 						pwd = "/path/to/"
// 					}
// 					data := `
// 						export let file = open("` + openPath + `");
// 						export default function() { return file };`

// 					sourceBuilder, err := getSimpleBuilder(filepath.ToSlash(filepath.Join(prefix, pwd, "script.js")), data, fs)
// 					if tCase.isError {
// 						assert.Error(t, err)
// 						return
// 					}
// 					require.NoError(t, err)

// 					arcBuilder, err := NewBuilderFromArchive(sourceBuilder.makeArchive(), lib.RuntimeOptions{})

// 					require.NoError(t, err)

// 					for source, b := range map[string]*Builder{"source": sourceBuilder, "archive": arcBuilder} {
// 						b := b
// 						t.Run(source, func(t *testing.T) {
// 							bi, err := b.Instantiate()
// 							require.NoError(t, err)
// 							v, err := bi.Default(goja.Undefined())
// 							require.NoError(t, err)
// 							assert.Equal(t, "hi", v.Export())
// 						})
// 					}
// 				}

// 				t.Run(tCase.name, testFunc)
// 				if isWindows {
// 					// windowsify the testcase
// 					tCase.openPath = strings.Replace(tCase.openPath, `/`, `\`, -1)
// 					tCase.pwd = strings.Replace(tCase.pwd, `/`, `\`, -1)
// 					t.Run(tCase.name+" with windows slash", testFunc)
// 				}
// 			}
// 		})
// 	}
// }

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

	bi, err := b.Build(nil)
	if !assert.NoError(t, err) {
		return
	}

	ctx := context.Background()
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
	env :=  map[string]string{
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

	bi, err := b.Build(nil)
	if assert.NoError(t, err) {
		_, err := bi.RunDefaultMethod(context.Background(), goja.Undefined())
		assert.NoError(t, err)
	}
}
