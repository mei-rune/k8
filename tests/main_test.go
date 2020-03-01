package tests

import (
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"github.com/runner-mei/goutils/urlutil"
	"github.com/runner-mei/k8"
	"github.com/runner-mei/moo"
	"github.com/runner-mei/moo/moo_tests"
	"golang.org/x/tools/godoc/vfs"
	"golang.org/x/tools/godoc/vfs/mapfs"
)

func TestSimple(t *testing.T) {
	appTest := moo_tests.NewAppTest(t)
	defer appTest.Close()

	moo.On(func() moo.Option {
		return moo.Provide(func() (vfs.NameSpace, k8.OutFiles) {
			fs := mapfs.New(map[string]string{
				"scripts/a.js": `module.exports.meta = {name: 'a'};
				module.exports.default = function(args) {
					return args.a + 1;
				}`,
			})
			ns := vfs.NewNameSpace()
			ns.Bind("/", fs, "/", vfs.BindAfter)

			return ns, k8.OutFiles{
				Filenames: []string{
					"/scripts/a.js",
				},
			}
		})
	})
	appTest.Start(t)

	urlStr := urlutil.Join(appTest.URL, appTest.Env.DaemonUrlPath, "/k8/a?a=b")
	res, err := http.Get(urlStr)
	if err != nil {
		t.Error(err)
		return
	}

	bs, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error(err)
		return
	}

	if res.StatusCode != http.StatusOK {
		t.Error(res.Status)
		t.Error(string(bs))
		return
	}

	s := string(bs)
	if !strings.Contains(s, "\"b1\"") {
		t.Error(s)
	} else {
		t.Log(s)
	}
}
