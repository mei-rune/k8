package main

// import (
// 	"cn/com/hengwei/pkg/environment"
// 	"log"

// 	"github.com/runner-mei/moo"
// 	_ "github.com/runner-mei/moo/auth/sessions/inmem"
// 	_ "github.com/runner-mei/moo/auth/users/db"
// )

// func init() {
// 	moo.On(func() moo.Option {
// 		return moo.Invoke(func(env *environment.Environment) {
// 			so := env.GetServiceConfig(environment.ENV_K8_PROXY_ID)
// 			network, addr := so.ListenAddr("", "")
// 			env.Config.Set("http-network", network)
// 			env.Config.Set("http-address", addr)
// 		})
// 	})
// }

// func main() {
// 	err := moo.Run(&moo.Arguments{
// 		Defaults: []string{
// 			"app.properties",
// 			"k8.properties",
// 			"engine.properties",
// 		},
// 		Customs: []string{
// 			"app.properties",
// 			"k8.properties",
// 			"engine.properties",
// 		},
// 		CommandArgs: []string{
// 			"moo.namespace=tpt",
// 			"product.name=k8",
// 			"daemon.urlpath=hengwei",
// 		},
// 	})
// 	if err != nil {
// 		log.Fatalln(err)
// 		return
// 	}
// }
