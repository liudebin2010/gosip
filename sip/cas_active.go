package sipapi

import (
	"fmt"
	"github.com/panjjo/gosip/db"
	sip "github.com/panjjo/gosip/sip/s"
	"github.com/panjjo/gosip/utils"
	"github.com/robfig/cron"
	"github.com/sirupsen/logrus"
	"strings"
)

func casKeepAliveCron() {
	c := cron.New()
	c.AddFunc("@every 1m", casKeepAliveFunc)
	c.AddFunc("@every 1h", casRegisterFunc)
	c.AddFunc("@every 5m", casCheckStreamFunc)
	c.AddFunc("@every 1h", sipMessageCatalogMysql)
	c.Start()
	casCheckStreamFunc()
}

func casKeepAliveFunc() {
	// from
	furi, _ := sip.ParseSipURI(fmt.Sprintf("sip:%s@%s", config.GB28181.LID, config.Cascade.LAddr))
	fromaddr := &sip.Address{
		URI:    &furi,
		Params: sip.NewParams(),
	}
	fromaddr.Params.Add("tag", sip.String{Str: utils.RandString(20)})
	// to
	turi, _ := sip.ParseSipURI(fmt.Sprintf("sip:%s@%s", config.Cascade.SID, config.Cascade.SUDP))
	toaddr := &sip.Address{
		URI:    &turi,
		Params: sip.NewParams(),
	}

	snLock.Lock()
	keepAliveSN++
	keepAliveSeq++
	if keepAliveSN > 10000000 {
		keepAliveSN = 10
	}
	if keepAliveSeq > 1000000 {
		keepAliveSeq = 1
	}
	snLock.Unlock()

	hb := sip.NewHeaderBuilder().SetTo(toaddr).SetFrom(fromaddr).AddVia(&sip.ViaHop{
		Params: sip.NewParams().Add("branch", sip.String{Str: sip.GenerateBranch()}),
	}).SetContentType(&sip.ContentTypeXML).SetMethod(sip.MESSAGE).SetSeqNo(keepAliveSeq)
	req := sip.NewRequest("", sip.MESSAGE, toaddr.URI, sip.DefaultSipVersion, hb.Build(), []byte(sip.GetKeepAliveXML(config.GB28181.LID, keepAliveSN)))
	req.SetDestination(sip.ReAddr)
	tx, err := cascadeSrv.CasRequest(req)
	if err != nil {
		logrus.Warn("keepalive request error, err=", err.Error())
		return
	}
	logrus.Infof("keepalive request, seq=%d", keepAliveSeq)

	_, err = cascadeSrv.CasSipResponse(tx)
	if err != nil {
		logrus.Warn("keepalive response error, err=", err.Error())
		// 如果接收心跳应答失败，重新发送注册消息
		casRegisterFunc()
		return
	}
	logrus.Infof("keepalive response, seq=%d", keepAliveSeq)
}

func casRegisterFunc() {
	casSendFirstRegister()
}

func casCheckStreamFunc() {
	logrus.Debug("check stream with cron every 5m")
	var skip, count int
	count = 100
	for {
		streams := []ChannelStream{}
		db.DBClient.Table(STREAMTB).Where("name = ?", "jinzhu").Find(&streams)
		db.FindT(db.DBClient, new(ChannelStream), &streams, db.M{"status = ?": 0}, "", skip, count, false)
		//db.DBClient.Find(STREAMTB, M, skip, count, "", false, &streams, nil)
		for _, stream := range streams {
			ssrc := stream.Stream
			ss := strings.Split(stream.Stream, "-")
			if len(ss) == 2 {
				ssrc = ss[1]
			}
			err := checkMediaList(ssrc)
			if err == nil {
				continue
			}
			logrus.Warnf("checkRtpInfo error, stop stream. stream=%s, ssrc=%d, time=%s, err=%s", stream.Stream, stream.SSRC, stream.Time, err.Error())
			closeChannelStream(stream.ChannelID, stream.CallID)
		}
		if len(streams) != int(count) {
			break
		}
		skip += count
	}
}
