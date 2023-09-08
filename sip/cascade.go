package sipapi

import (
	sip "github.com/panjjo/gosip/sip/s"
	"time"
)

// Cascade Cascade
type Cascade struct {
	SID      string `json:"sid" yaml:"sid"`
	Region   string `json:"region" yaml:"region"`
	SUDP     string `json:"sudp" yaml:"sudp"`
	SPWD     string `json:"spwd" yaml:"spwd"`
	LAddr    string `json:"laddr" yaml:"laddr"`
	LUDP     string `json:"ludp" yaml:"ludp"`
	LTcp     string `json:"ltcp" yaml:"ltcp"`
	Run      int    `json:"run" yaml:"run"`
	Sport    int    `json:"sport" yaml:"sport"`
	Eport    int    `json:"eport" yaml:"eport"`
	CityID   string `json:"cityid" yaml:"cityid"`
	CityName string `json:"cityname" yaml:"cityname"`
	CataMod  int    `json:"catamod" yaml:"catamod"`
}

// PlayCtl play control
type PlayCtl struct {
	// UpNum up stream num
	UpNum int `json:"upnum" yaml:"upnum"`
	// DownNum down stream num
	DownNum int `json:"downnum" yaml:"downnum"`
}

var (
	// sip服务用户信息
	cassrv *sip.Server
)

func cascadeInit() {
	// 创建级联服务
	time.Sleep(time.Duration(2) * time.Second)
	cassrv = sip.CasNewServer()
	cassrv.RegistHandler(sip.REGISTER, casHandlerRegister)
	cassrv.RegistHandler(sip.MESSAGE, casHandlerMessage)
	cassrv.RegistHandler(sip.SUBSCRIBE, casHandlerSubscribe)
	cassrv.RegistHandler(sip.INVITE, casHandlerInvite)
	cassrv.RegistHandler(sip.INFO, casHandlerInfo)
	cassrv.RegistHandler(sip.ACK, casHandlerAck)
	cassrv.RegistHandler(sip.BYE, casHandlerBye)
	cassrv.CreateCasUDPServer(config.Cascade.SUDP, config.Cascade.LUDP)
	casSendFirstRegister()
	casKeepAliveCron()
	go cassrv.ListenCasUDPServer()
	// go CasRestfulAPI()
}
