package main

import (
	"github.com/gin-gonic/gin"
	"github.com/panjjo/gosip/api"
	"github.com/panjjo/gosip/api/middleware"
	_ "github.com/panjjo/gosip/docs"
	"github.com/panjjo/gosip/m"
	sipapi "github.com/panjjo/gosip/sip"
	"github.com/sirupsen/logrus"
	"net/http"
	_ "net/http/pprof"

	"github.com/robfig/cron"
	swaggerfiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// @title          GoSIP
// @version        2.0
// @description    GB28181 SIP服务端.
// @termsOfService https://github.com/panjjo/gosip

// @contact.name  GoSIP
// @contact.url   https://github.com/panjjo/gosip
// @contact.email panjjo@vip.qq.com

// @license.name Apache 2.0
// @license.url  http://www.apache.org/licenses/LICENSE-2.0.html

// @host     localhost:8090
// @BasePath /

// @securityDefinitions.basic BasicAuth

func main() {
	//pprof
	//访问地址：http://0.0.0.0:6060/debug/pprof
	go func() {
		http.ListenAndServe("0.0.0.0:6060", nil)
	}()

	sipapi.Start()

	//构建swagger文档
	//访问地址： http://0.0.0.0:6060/swagger/index.html
	r := gin.Default()
	r.Use(middleware.Recovery)
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerfiles.Handler))

	//开放restfull api
	api.Init(r)

	logrus.Fatal(r.Run(m.MConfig.API))
}

// init函数先于main函数自动执行
func init() {
	m.LoadConfig()
	// 服务启动时将ZLM的回调写到ZLM服务器配置文件上
	sipapi.SyncWebhook2ZlmConfig()
	_cron()
}

// 定时任务
func _cron() {
	c := cron.New()                                 // 新建一个定时任务对象
	c.AddFunc("0 */5 * * * *", sipapi.CheckStreams) // 定时关闭推送流
	c.AddFunc("0 */5 * * * *", sipapi.ClearFiles)   // 定时清理录制文件
	c.Start()
}
