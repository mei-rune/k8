package tests

import (
	"io/fs"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/runner-mei/k8"
	"tech.hengwei.com.cn/go/goutils/urlutil"
	"tech.hengwei.com.cn/go/moo"
	"tech.hengwei.com.cn/go/moo/authn"
	"tech.hengwei.com.cn/go/moo/moo_simple_tests"

	_ "tech.hengwei.com.cn/go/moo/authn/sessions/inmem"
	_ "tech.hengwei.com.cn/go/moo/authn/users/fileusers"
)

func init() {
	authn.SetDefaultClientName("moo")
}

func TestSimple(t *testing.T) {
	appTest := moo_simple_tests.NewTestApp(t)
	defer appTest.Close()

	moo.On(func(*moo.Environment) moo.Option {
		return moo.Provide(func() (fs.FS, k8.OutFiles) {
			scriptFS := fstest.MapFS{
				"scripts/a.js": &fstest.MapFile{
					Data: []byte(`module.exports.meta = {id: 'a', name: 'a'};
					module.exports.default = function(args) {
						return args.a + 1;
					}`),
				},
			}

			return scriptFS, k8.OutFiles{
				Filenames: []string{
					"scripts/a.js",
				},
			}
		})
	})
	appTest.Start(t)

	urlStr := urlutil.Join(appTest.URL, appTest.Env.DaemonUrlPath, "/k8/meta/methods")
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
	if !strings.Contains(s, "\"name\":\"a\"") {
		t.Error(s)
	} else {
		t.Log(s)
	}

	urlStr = urlutil.Join(appTest.URL, appTest.Env.DaemonUrlPath, "/k8/a?a=b")
	res, err = http.Get(urlStr)
	if err != nil {
		t.Error(err)
		return
	}

	bs, err = ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error(err)
		return
	}

	if res.StatusCode != http.StatusOK {
		t.Error(res.Status)
		t.Error(string(bs))
		return
	}

	s = string(bs)
	if !strings.Contains(s, "\"b1\"") {
		t.Error(s)
	} else {
		t.Log(s)
	}
}
