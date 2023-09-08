package sipapi

import (
	"fmt"
	"github.com/panjjo/gosip/db"
	sip "github.com/panjjo/gosip/sip/s"
	"github.com/panjjo/gosip/utils"
	"github.com/sirupsen/logrus"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	sdp "github.com/panjjo/gosdp"
)

const (
	// 注册过期时间
	EXPIRESTIME = 3600
	USERAGENT   = "GOSIP_PROXY_V2.0"
	RTPKEY      = "tcp-active"
)

var (
	// SN 序列号
	keepAliveSN          int64
	snLock, msgLock      *sync.Mutex
	keepAliveSeq, msgSeq uint
	regSeq               uint
	// ackMap               *sync.Map
	// 历史视频请求，保存callid，防止同时到达多个相同请求
	hisCallMap *sync.Map
	// 所有视频请求，前端设备id，防止同时对同个设备取流
	videoDevIdMap *sync.Map
	// 上级平台tcp主动模式取流，串行接受连接，推流
	// 专为上海级联设计，不具有通用性
	sendRtpMap *sync.Map
	// tcp被动模式下，监听端口区间取值 2022/9/9 中秋节
	portLock *sync.Mutex
	lastPort int
	// 街道信息
	streetMap *sync.Map
)

func init() {
	keepAliveSN = 10
	snLock = &sync.Mutex{}
	ssrcMap = &sync.Map{}
	msgSeq = 10
	msgLock = &sync.Mutex{}
	rtpSeq = 1
	rtpLock = &sync.Mutex{}
	regSeq = 1
	callIDMap = &sync.Map{}
	hisCallMap = &sync.Map{}
	videoDevIdMap = &sync.Map{}
	playbackMap = &sync.Map{}
	ffmpegMap = &sync.Map{}
	sendRtpMap = &sync.Map{}
	nvrRecord = &sync.Map{}
	portLock = &sync.Mutex{}
	streetMap = &sync.Map{}
}

func casHandlerMessage(req *sip.Request, tx *sip.Transaction) {
	logrus.Debugf("cas message req, str=\n%s", req.String())
	u, ok := parserDevicesFromReqeust(req)
	if !ok {
		// 未解析出来源用户返回错误
		tx.CasResponse(sip.NewResponseFromRequest("", req, http.StatusBadRequest, http.StatusText(http.StatusBadRequest), []byte("")))
		logrus.Error("message parse request failed")
		return
	}
	// 判断是否存在body数据
	if len, have := req.ContentLength(); !have || len.Equals(0) {
		// 不存在就直接返回的成功
		tx.CasResponse(sip.NewResponseFromRequest("", req, http.StatusOK, "", []byte("")))
		logrus.Error("cas message parse request not find body")
		return
	}
	body := req.Body()
	message := &MessageReceive{}
	if err := utils.XMLDecode([]byte(body), message); err != nil {
		logrus.Errorf("handler message Unmarshal xml err:%s, body:\n%s", err.Error(), body)
		tx.CasResponse(sip.NewResponseFromRequest("", req, http.StatusBadRequest, http.StatusText(http.StatusBadRequest), []byte("")))
		return
	}

	logrus.Infof("cas message request, cmd type=%s, deviceid=%s", message.CmdType, u.DeviceID)
	switch message.CmdType {
	case "Catalog":
		// 设备列表
		tx.CasResponse(sip.NewResponseFromRequest("", req, http.StatusOK, http.StatusText(http.StatusOK), []byte("")))
		if config.Cascade.Run == 1 {
			sipMessageCatalogMongo(u, string(body))
		} else if config.Cascade.Run == 2 {
			sipMessageCatalogNvrMysql(message.SN)
		}
		return
	case "Keepalive":
		// heardbeat
		tx.CasResponse(sip.NewResponseFromRequest("", req, http.StatusOK, http.StatusText(http.StatusOK), []byte("")))
		return
	case "RecordInfo":
		// 设备音视频文件列表
		rec := &MessageRecordInfoRequest{}
		if err := utils.XMLDecode([]byte(body), rec); err != nil {
			logrus.Errorf("RecordInfo Unmarshal xml err:%s, body:\n%s", err.Error(), body)
			tx.CasResponse(sip.NewResponseFromRequest("", req, http.StatusBadRequest, http.StatusText(http.StatusBadRequest), []byte("")))
			return
		}
		s, _ := time.ParseInLocation("2006-01-02T15:04:05", rec.StartTime, time.Local)
		e, _ := time.ParseInLocation("2006-01-02T15:04:05", rec.EndTime, time.Local)
		if s.Unix() >= e.Unix() {
			tx.CasResponse(sip.NewResponseFromRequest("", req, http.StatusBadRequest, http.StatusText(http.StatusBadRequest), []byte("")))
			return
		}
		tx.CasResponse(sip.NewResponseFromRequest("", req, http.StatusOK, http.StatusText(http.StatusOK), []byte("")))
		// 向下级相机请求文件列表
		// devID := req.Recipient().User().String()
		err := sipCasRecordList(u, message.DeviceID, string(body))
		if err != nil {
			logrus.Errorf("cas recordinfo error, err:%s", err.Error())
		}
		return
	}
	tx.CasResponse(sip.NewResponseFromRequest("", req, http.StatusOK, http.StatusText(http.StatusOK), []byte("")))
}

func casHandlerSubscribe(req *sip.Request, tx *sip.Transaction) {
	logrus.Debugf("cas subscribe req, str=\n%s", req.String())
	u, ok := parserDevicesFromReqeust(req)
	if !ok {
		// 未解析出来源用户返回错误
		tx.CasResponse(sip.NewResponseFromRequest("", req, http.StatusBadRequest, http.StatusText(http.StatusBadRequest), []byte("")))
		logrus.Error("subscribe parse request failed")
		return
	}
	// 判断是否存在body数据
	if len, have := req.ContentLength(); !have || len.Equals(0) {
		// 不存在就直接返回的成功
		tx.CasResponse(sip.NewResponseFromRequest("", req, http.StatusOK, "", []byte("")))
		logrus.Error("subscribe parse request not find body")
		return
	}
	body := req.Body()
	message := &MessageReceive{}
	if err := utils.XMLDecode([]byte(body), message); err != nil {
		logrus.Errorf("handler subscribe Unmarshal xml err:%s, body:\n%s", err.Error(), body)
		tx.CasResponse(sip.NewResponseFromRequest("", req, http.StatusBadRequest, http.StatusText(http.StatusBadRequest), []byte("")))
		return
	}

	logrus.Infof("cas subscribe request, cmd type=%s, deviceid=%s", message.CmdType, u.DeviceID)
	switch message.CmdType {
	case "Catalog":
		// 设备列表
		tx.CasResponse(sip.NewResponseFromRequest("", req, http.StatusOK, http.StatusText(http.StatusOK), []byte("")))
		if config.Cascade.Run == 1 {
			sipMessageCatalogMongo(u, string(body))
		} else if config.Cascade.Run == 2 {
			sipMessageCatalogNvrMysql(message.SN)
		}
		return
	}
	tx.CasResponse(sip.NewResponseFromRequest("", req, http.StatusOK, http.StatusText(http.StatusOK), []byte("")))
}

func casSendFirstRegister() {
	randStr := utils.RandString(16)
	registerRequest := fmt.Sprintf(
		"REGISTER sip:%s@%s SIP/2.0\r\n"+
			"Via: SIP/2.0/UDP %s;rport;branch=z9hG4bK%s\r\n"+
			"Max-Forwards: 70\r\n"+
			"To: <sip:%s@%s>\r\n"+
			"From: <sip:%s@%s>;tag=456789%s\r\n"+
			"Call-ID: %s\r\n"+
			"CSeq: %d REGISTER\r\n"+
			"Contact: <sip:%s@%s>\r\n"+
			"Expires: %d\r\n"+
			"User-Agent: %s\r\n"+
			"Content-Length: 0\r\n\r\n",
		config.Cascade.SID, config.Cascade.SUDP,
		config.Cascade.LAddr, randStr,
		config.GB28181.LID, config.Cascade.LAddr,
		config.GB28181.LID, config.Cascade.LAddr, randStr,
		randStr, regSeq,
		config.GB28181.LID, config.Cascade.LAddr,
		EXPIRESTIME, USERAGENT)

	cassrv.CasWrite([]byte(registerRequest))
	logrus.Infof("first register, from id:%s, to id:%s", config.GB28181.LID, config.Cascade.SID)
	logrus.Debugf("first register, str:\n%s", registerRequest)
	cassrv.CasNewPacket([]byte(registerRequest), sip.LoAddr)
}

func casHandlerRegister(req *sip.Request, tx *sip.Transaction) {
	response, err := cassrv.GetCasRegResponse(tx)
	if err != nil {
		logrus.Errorf("first register get response failed, err=%s", err.Error())
		return
	}
	logrus.Info("first register response")
	logrus.Debugf("first register response, str:\n%s", response.String())

	if response.StatusCode() != 401 || response.Reason() != "Unauthorized" {
		logrus.Warnf("register response error, code=%d, reason=%s", response.StatusCode(), response.Reason())
		// 如果上级平台不进行校验，直接发送心跳
		if response.StatusCode() == 200 {
			casKeepAliveFunc()
			// rtsp方式拉流，且主动向上级推送设备目录信息
			if config.Cascade.CataMod == 1 && config.Cascade.Run == 2 {
				sipMessageCatalogMysql()
			}
		}
		return
	}

	hdrs := response.GetHeaders("WWW-Authenticate")
	if len(hdrs) == 0 {
		logrus.Warn("register response could not find WWW-Authenticate")
		return
	}
	authHd := hdrs[0].(*sip.GenericHeader)
	auth := sip.AuthFromValue(authHd.Contents)

	// digest access authentication
	uri := fmt.Sprintf("sip:%s@%s", config.Cascade.SID, config.Cascade.SUDP)
	HA1 := config.GB28181.LID + ":" + auth.Get("realm") + ":" + config.Cascade.SPWD
	HA2 := "REGISTER" + ":" + uri
	authResp := utils.GetMD5(utils.GetMD5(HA1) + ":" + auth.Get("nonce") + ":" + utils.GetMD5(HA2))

	secReq := sip.NewRequestFromResponse(sip.REGISTER, response)
	authStr := fmt.Sprintf("Digest username=\"%s\", realm=\"%s\", nonce=\"%s\", uri=\"%s\", response=\"%s\", algorithm=MD5", config.GB28181.LID, auth.Get("realm"), auth.Get("nonce"), uri, authResp)
	secReq.AppendHeader(&sip.GenericHeader{HeaderName: "Authorization", Contents: authStr})
	secReq.AppendHeader(&sip.GenericHeader{HeaderName: "User-Agent", Contents: USERAGENT})
	secReq.AppendHeader(&sip.GenericHeader{HeaderName: "Content-Length", Contents: "0"})
	secReq.AppendHeader(&sip.GenericHeader{HeaderName: "Expires", Contents: strconv.Itoa(EXPIRESTIME)})

	cascadeSrv.CasWrite([]byte(secReq.String()))
	logrus.Infof("second register request, uri:%s", uri)
	logrus.Debugf("second register request, str:\n%s", secReq.String())
	sq, _ := secReq.CSeq()
	regSeq = uint(sq.SeqNo) + 1

	response, _ = cascadeSrv.CasSipResponse(tx)
	if response != nil {
		logrus.Info("second register response")
		logrus.Debugf("second register response, str:\n%s", response.String())
	}
	casKeepAliveFunc()
	if config.Cascade.CataMod == 1 && config.Cascade.Run == 2 {
		sipMessageCatalogMysql()
	}
}

func casHandlerInvite(req *sip.Request, tx *sip.Transaction) {
	logrus.Infof("cas invite request, str:\n%s", req.String())

	callID, _ := req.CallID()
	// 防止相同callid请求
	if _, ok := hisCallMap.Load(string(*callID)); ok {
		logrus.Warnf("BadRequest cas invite request, callid is exist, id:%s", string(*callID))
		tx.CasResponse(sip.NewResponseFromRequest("", req, http.StatusBadRequest, http.StatusText(http.StatusBadRequest), []byte("")))
		return
	}
	_, ok := parserDevicesFromReqeust(req)
	if !ok {
		// 未解析出来源用户返回错误
		tx.CasResponse(sip.NewResponseFromRequest("", req, http.StatusBadRequest, http.StatusText(http.StatusBadRequest), []byte("")))
		logrus.Error("BadRequest invite parse request failed")
		return
	}
	// 判断是否存在body数据
	if len, have := req.ContentLength(); !have || len.Equals(0) {
		// 不存在就直接返回的成功
		tx.CasResponse(sip.NewResponseFromRequest("", req, http.StatusBadRequest, http.StatusText(http.StatusBadRequest), []byte("")))
		logrus.Error("BadRequest no body")
		return
	}

	deviceID := req.Recipient().User().String()
	// 判断相机是否正在被使用
	if _, ok := videoDevIdMap.Load(deviceID); ok {
		logrus.Warnf("BadRequest cas invite request, device is being used, id:%s", deviceID)
		tx.CasResponse(sip.NewResponseFromRequest("", req, http.StatusBadRequest, http.StatusText(http.StatusBadRequest), []byte("")))
		return
	}
	// 播放逻辑控制
	ok, _ = canPlay(deviceID)
	if !ok {
		// CANCEL
		tx.CasResponse(sip.NewResponseFromRequest("", req, http.StatusBadRequest, http.StatusText(http.StatusBadRequest), []byte("")))
		logrus.Warn("BadRequest too many playing streams, cancel")
		return
	}
	devices := Devices{}
	// 判断是sip注册或者rtsp流
	// streamType := 1 // 流类型，1-sip，2-rtsp
	cnt, _ := db.FindT(db.DBClient, new(Devices), &devices, db.M{"deviceid": deviceID, "active> ?": time.Now().Unix() - 1800}, "", 0, 100, true)
	//cnt, _ := dbClient.Count(deviceTB, M{"deviceid": deviceID, "active": M{"$gt": time.Now().Unix() - 1800}})
	// sip注册
	if cnt > 0 {
		logrus.Debugf("Invite find device, id=%s", deviceID)
	}
	//
	// trying response
	resp := createTryingResponse(req)
	err := tx.CasResponse(resp)
	if err != nil {
		logrus.Errorf("ServerError send trying response failed, err=%s", err.Error())
		tx.CasResponse(sip.NewResponseFromRequest("", req, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError), []byte("")))
		return
	}
	logrus.Info("send trying response")

	body := req.Body()
	m, err := sdp.Decode([]byte(body))
	if err != nil {
		logrus.Errorf("BadRequest deocde sdp failed, err:%s, body:\n%s", err.Error(), body)
		tx.CasResponse(sip.NewResponseFromRequest("", req, http.StatusBadRequest, http.StatusText(http.StatusBadRequest), []byte("")))
		return
	}
	if len(m.Medias) == 0 {
		logrus.Warnf("BadRequest media data is null, err:%s, body:\n%s", err.Error(), body)
		tx.CasResponse(sip.NewResponseFromRequest("", req, http.StatusBadRequest, http.StatusText(http.StatusBadRequest), []byte("")))
		return
	}
	media := m.Medias[0]
	hisCallMap.Store(string(*callID), 1)
	videoDevIdMap.Store(deviceID, 1)

	// 判断该是否在推流
	var (
		bPushed  bool
		d        playParams
		ssrc     string
		filesize string
		// snum     int // 当前通道推流数量
	)
	// 判断是否重复取流
	ssrcData, ok := ssrcMap.Load(deviceID)
	// snum = data.(int)
	if config.Cascade.Run == 1 {
		d = playParams{S: time.Time{}, E: time.Time{}, DeviceID: deviceID, UserID: "", up: true}
		if ok && m.Name == "Play" { // 直播流可以复用
			bPushed = true
			d.cstream = media.Attributes.Value("stream")
		} else {
			// 向注册相机请求视频流
			// d = playParams{S: time.Time{}, E: time.Time{}, DeviceID: deviceID, UserID: "", up: true}
			if m.Name == "Playback" {
				d.T = 1
				d.S = m.Timing[0].Start
				d.E = m.Timing[0].End
			} else if m.Name == "Download" {
				d.T = 2
				d.S = m.Timing[0].Start
				d.E = m.Timing[0].End
				d.dlspeed = media.Attributes.Value("downloadspeed")
			} else if m.Name == "Play" {
				d.cstream = media.Attributes.Value("stream")
			}
			ssrc, err = inviteStreamFromDevice(d)
			if err != nil {
				logrus.Errorf("NotFound inviteStreamFromDevice failed, err=%s", err.Error())
				tx.CasResponse(sip.NewResponseFromRequest("", req, http.StatusNotFound, http.StatusText(http.StatusNotFound), []byte("")))
				hisCallMap.Delete(string(*callID))
				videoDevIdMap.Delete(deviceID)
				return
			}
			if m.Name == "Download" { // 解析
				if data, ok := _playList.ssrcResponse.Load(ssrc); ok {
					play := data.(playParams)
					body := play.Resp.Body()
					m2, err := sdp.Decode([]byte(body))
					if err == nil && len(m2.Medias) > 0 {
						media2 := m2.Medias[0]
						filesize = media2.Attributes.Value("filesize")
					}
				}
			}
			hisCallMap.Store(string(*callID), ssrc)
		}
		// 进行转码，转换为子码流
		if ssrc == "" {
			n := 0
			for {
				if n > 10 {
					logrus.Warnf("cas invite request, ssrc is nil, did:%s", deviceID)
					hisCallMap.Delete(string(*callID))
					videoDevIdMap.Delete(deviceID)
					tx.CasResponse(sip.NewResponseFromRequest("", req, http.StatusConflict, http.StatusText(http.StatusConflict), []byte("")))
					return
				}
				_, ok := ssrcMap.Load(deviceID)
				if !ok {
					break
				}
				n++
				time.Sleep(time.Millisecond * 100)
			}
			ssrc, err = inviteStreamFromDevice(d)
			if err != nil {
				logrus.Errorf("NotFound inviteStreamFromDevice failed, err=%s", err.Error())
				tx.CasResponse(sip.NewResponseFromRequest("", req, http.StatusNotFound, http.StatusText(http.StatusNotFound), []byte("")))
				hisCallMap.Delete(string(*callID))
				videoDevIdMap.Delete(deviceID)
				return
			}
		}
		// 所有请求在此等待，适应于上级平台tcp主动取流模式
		for {
			if _, ok := sendRtpMap.LoadOrStore(RTPKEY, 1); !ok {
				logrus.Debugf("continue sending stream, callid:%s, devid:%s, ssrc:%s", string(*callID), deviceID, ssrc)
				break
			}
			time.Sleep(time.Millisecond * 10)
		}
	}

	ps := config.Cascade.LTcp
	portLock.Lock()
	if lastPort < config.Cascade.Sport || lastPort >= config.Cascade.Eport {
		lastPort = config.Cascade.Sport
	} else {
		lastPort++
	}
	myport := lastPort
	portLock.Unlock()

	v := sdp.Media{
		Description: sdp.MediaDescription{
			Type:     "video",
			Port:     myport,
			Protocol: media.Description.Protocol,
			Formats:  []string{"96"},
		},
	}
	isUDP := 1
	active := false
	if media.Description.Protocol == "TCP/RTP/AVP" {
		st := media.Attributes.Value("setup")
		if st == "passive" {
			v.AddAttribute("setup", "active")
		} else if st == "active" {
			v.AddAttribute("setup", "passive")
			active = true
		}
		v.AddAttribute("connection", "new")
		isUDP = 0
	}
	v.AddAttribute("sendonly")
	v.AddAttribute("rtpmap", "96", "PS/90000")
	if d.T == 2 {
		v.AddAttribute("filesize", filesize)
	}

	mm := &sdp.Message{
		Origin: sdp.Origin{
			Username:       deviceID,
			SessionID:      m.Origin.SessionID,
			SessionVersion: m.Origin.SessionVersion,
			Address:        ps,
		},
		Name: m.Name,
		Connection: sdp.ConnectionData{
			IP: net.ParseIP(ps),
		},
		// Timing: m.Timing,
		Timing: []sdp.Timing{{Start: m.Timing[0].Start, End: m.Timing[0].End}},
		Medias: []sdp.Media{v},
		SSRC:   m.SSRC,
	}
	var (
		s sdp.Session
		b []byte
	)
	s = mm.Append(s)
	b = s.AppendTo(b)

	// 通道流信息
	cs := &ChannelStream{}
	cs.ChannelID = deviceID
	cs.StreamType = media.Attributes.Value("stream")
	// 下级被动推流
	if !ok && config.Cascade.Run == 2 {
		err = sendRtspRtpServer(isUDP, cs)
		if err != nil {
			logrus.Errorf("sendRtspRtpServer error, err=%s", err.Error())
			tx.CasResponse(sip.NewResponseFromRequest("", req, http.StatusNotFound, err.Error(), []byte("")))
			hisCallMap.Delete(string(*callID))
			videoDevIdMap.Delete(deviceID)
			return
		}
	}

	resp = sip.NewResponseFromRequest("", req, http.StatusOK, http.StatusText(http.StatusOK), b)
	to, _ := resp.To()
	to.Params.Add("tag", sip.String{Str: utils.RandString(10)})
	// Contact: <sip:31010000001318000101@192.168.8.168:5060>
	resp.AppendHeader(&sip.GenericHeader{HeaderName: "Contact", Contents: "<" + req.Recipient().String() + ">"})
	resp.AppendHeader(&sip.GenericHeader{HeaderName: "User-Agent", Contents: USERAGENT})
	resp.AppendHeader(&sip.GenericHeader{HeaderName: "Content-Type", Contents: "application/sdp"})

	logrus.Infof("cas invite response, str:\n%s", resp.String())
	err = tx.CasResponse(resp)
	if err != nil {
		logrus.Errorf("cas invite response failed, err=%s", err.Error())
		hisCallMap.Delete(string(*callID))
		videoDevIdMap.Delete(deviceID)
		sendRtpMap.Delete(RTPKEY)
		return
	}

	// sendRtp
	host := m.Connection.IP.String()
	port := media.Description.Port
	// 判断该通道是否在推流
	if bPushed && d.cstream != "1" { // 主码流直接推送
		c := &ChannelStream{}

		//_ = dbClient.Get(STREAMTB, M{"cid": deviceID, "stop": false, "playt": 0}, c)
		c.Host = host
		c.Port = port
		c.CallID = string(*callID)
		c.TcpPort = myport

		s := c.Stream // 父ssrc
		ss := strings.Split(c.Stream, "-")
		if len(ss) == 2 {
			s = ss[1]
		}
		err = sendNewRtpServer(isUDP, c, s)
		num := ssrcData.(int)
		if err == nil {
			num++
			ssrcMap.Store(deviceID, num)
		}
		logrus.Infof("send rtp server, deviceid=%s, host=%s, port=%d, callid=%s, stream num=%d", deviceID, host, port, string(*callID), num)
		videoDevIdMap.Delete(deviceID)
		sendRtpMap.Delete(RTPKEY)
		return
	}
	// rtsp协议，重复取流
	if ok && config.Cascade.Run == 2 {
		c := &ChannelStream{}

		//_ = db.Get(STREAMTB, M{"cid": deviceID, "stop": false, "playt": 0}, c)
		c.Host = host
		c.Port = port
		c.CallID = string(*callID)
		c.TcpPort = myport
		err = sendNewRtpServer(isUDP, c, c.Stream)
		num := ssrcData.(int)
		if err == nil {
			num++
			ssrcMap.Store(deviceID, num)
		}
		logrus.Infof("rtsp send rtp server, deviceid=%s, host=%s, port=%d, callid=%s, stream num=%d", deviceID, host, port, string(*callID), num)
		videoDevIdMap.Delete(deviceID)
		return
	}

	// 通道流信息
	// cs := &ChannelStream{}
	cs.PlayT = d.T
	cs.Port = port
	cs.CallID = string(*callID)
	cs.ChannelID = deviceID
	cs.Host = host
	if active {
		cs.TcpPort = myport
	}
	cs.IsActive = active
	// sip
	if config.Cascade.Run == 1 {
		cs.Stream = ssrc
		_ = sendRtpServer(isUDP, cs, d)
	} else if config.Cascade.Run == 2 {
		// rtsp
		// cs.StreamType = media.Attributes.Value("stream")
		// _ = sendRtspRtpServer(isUDP, cs)
		err = sendNewRtpServer(isUDP, cs, cs.Stream)
		if err == nil {
			ssrcMap.Store(cs.ChannelID, 1)
		}
	}
	sendRtpMap.Delete(RTPKEY)
	// 推流成功后，删除相机使用记录，此时可以复用相机推流
	videoDevIdMap.Delete(deviceID)
}

func casHandlerInfo(req *sip.Request, tx *sip.Transaction) {
	logrus.Infof("cas info handler, str:\n%s", req.String())

	var ssrc string
	callID, _ := req.CallID()
	if v, ok := hisCallMap.Load(string(*callID)); ok {
		ssrc = v.(string)
	}
	did := req.Recipient().User().String()
	body, err := sipPlayback(did, ssrc, string(req.Body()))
	if err != nil {
		logrus.Errorf("sipPlayback failed, err:%s", err.Error())
		tx.CasResponse(sip.NewResponseFromRequest("", req, http.StatusOK, http.StatusText(http.StatusOK), []byte("")))
		return
	}
	tx.CasResponse(sip.NewResponseFromRequest("", req, http.StatusOK, http.StatusText(http.StatusOK), []byte(body)))
	logrus.Infof("cas info handler response, callid:%s", string(*callID))
}

func casHandlerAck(req *sip.Request, tx *sip.Transaction) {
	logrus.Debugf("cas ack response, str:\n%s", req.String())
	callID, _ := req.CallID()
	logrus.Info("cas ack handler, callid=", string(*callID))
}

func casHandlerBye(req *sip.Request, tx *sip.Transaction) {
	logrus.Info("cas bye handler")
	logrus.Debugf("cas bye request, str:\n%s", req.String())

	to, _ := req.To()
	deviceID := to.Address.User().String()
	callID, _ := req.CallID()
	hisCallMap.Delete(string(*callID))
	closeChannelStream(deviceID, string(*callID))

	tx.CasResponse(sip.NewResponseFromRequest("", req, http.StatusOK, http.StatusText(http.StatusOK), []byte("")))
	logrus.Infof("cas bye handler finished, callid:%s", string(*callID))
}
