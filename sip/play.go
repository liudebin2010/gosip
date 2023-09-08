package sipapi

import (
	"errors"
	"fmt"
	"sync"
	"time"

	sdp "github.com/panjjo/gosdp"
	"github.com/panjjo/gosip/db"
	"github.com/panjjo/gosip/m"
	sip "github.com/panjjo/gosip/sip/s"
	"github.com/panjjo/gosip/utils"
	"github.com/sirupsen/logrus"
)

// sip 请求播放
func SipPlay(data *Streams) (*Streams, error) {

	channel := Channels{ChannelID: data.ChannelID}
	if err := db.Get(db.DBClient, &channel); err != nil {
		if db.RecordNotFound(err) {
			return nil, errors.New("通道不存在")
		}
		return nil, err
	}

	data.DeviceID = channel.DeviceID
	data.StreamType = channel.StreamType
	// 使用通道的播放模式进行处理
	switch channel.StreamType {
	case m.StreamTypePull:
		// 拉流

	default:
		// 推流模式要求设备在线且活跃
		if time.Now().Unix()-channel.Active > 30*60 || channel.Status != m.DeviceStatusON {
			return nil, errors.New("通道已离线")
		}
		user, ok := _activeDevices.Get(channel.DeviceID)
		if !ok {
			return nil, errors.New("设备已离线")
		}
		// GB28181推流
		if data.StreamID == "" {
			ssrcLock.Lock()
			data.ssrc = getSSRC(data.T)
			data.StreamID = ssrc2stream(data.ssrc)

			// 成功后保存
			db.Create(db.DBClient, data)
			ssrcLock.Unlock()
		}

		var err error
		data, err = sipPlayPush(data, channel, user)
		if err != nil {
			return nil, fmt.Errorf("获取视频失败:%v", err)
		}
	}

	data.HTTP = fmt.Sprintf("%s/rtp/%s/hls.m3u8", config.Media.HTTP, data.StreamID)
	data.RTMP = fmt.Sprintf("%s/rtp/%s", config.Media.RTMP, data.StreamID)
	data.RTSP = fmt.Sprintf("%s/rtp/%s", config.Media.RTSP, data.StreamID)
	data.WSFLV = fmt.Sprintf("%s/rtp/%s.live.flv", config.Media.WS, data.StreamID)

	data.Ext = time.Now().Unix() + 2*60 // 2分钟等待时间
	StreamList.Response.Store(data.StreamID, data)
	if data.T == 0 {
		StreamList.Succ.Store(data.ChannelID, data)
	}
	db.Save(db.DBClient, data)
	return data, nil
}

var ssrcLock *sync.Mutex

func sipCasPlayPush(data playParams, device DeviceItem, user Devices) (playParams, error) {
	var (
		s sdp.Session
		b []byte
	)
	name := "Play"
	protocal := "TCP/RTP/AVP"
	if data.T == 1 { // 历史视频
		name = "Playback"
		protocal = "RTP/RTCP"
	} else if data.T == 2 { // 下载
		name = "Download"
		// protocal = "TCP/RTP/AVP"
	}
	if data.SSRC == "" {
		ssrcLock.Lock()
		data.SSRC = getSSRC(data.T)
		ssrcLock.Unlock()
	}
	// 获取zlm流媒体服务器地址
	ssrc := ssrc2stream(data.SSRC)
	if len(ssrc) == 7 {
		ssrc = "0" + ssrc
	}
	//ip, port, err := zlmDis.GetZlmAddr(ssrc)
	//if err != nil {
	//	logrus.Error("get zlm addr failed, err=", err)
	//	return data, err
	//}

	video := sdp.Media{
		Description: sdp.MediaDescription{
			Type:     "video",
			Port:     _sysinfo.MediaServerRtpPort, //port
			Formats:  []string{"96", "98", "97"},
			Protocol: protocal,
		},
	}
	video.AddAttribute("recvonly")
	if data.T == 0 || data.T == 2 {
		video.AddAttribute("setup", "passive")
		video.AddAttribute("connection", "new")
	}
	video.AddAttribute("rtpmap", "96", "PS/90000")
	video.AddAttribute("rtpmap", "98", "H264/90000")
	video.AddAttribute("rtpmap", "97", "MPEG4/90000")
	var codec string
	if data.T == 0 {
		if len(data.cstream) == 0 {
			data.cstream = "0"
		}
		video.AddAttribute("stream", data.cstream)
	} else if data.T == 2 {
		video.AddAttribute("downloadspeed", data.dlspeed)
	}

	// defining message
	msg := &sdp.Message{
		Origin: sdp.Origin{
			Username: _serverDevices.DeviceID,           // SIP服务器id
			Address:  string(_sysinfo.MediaServerRtpIP), //ip,
		},
		Name: name,
		Connection: sdp.ConnectionData{
			IP:  _sysinfo.MediaServerRtpIP, //ip, //_sysinfo.mediaServerRtpIP,
			TTL: 0,
		},
		Timing: []sdp.Timing{{Start: data.S, End: data.E}},
		Medias: []sdp.Media{video},
		SSRC:   data.SSRC,
		Codec:  codec,
	}
	if data.T == 1 || data.T == 2 {
		msg.URI = fmt.Sprintf("%s:0", data.DeviceID)
	}

	// appending message to session
	s = msg.Append(s)
	// appending session to byte buffer
	b = s.AppendTo(b)
	deviceURI, _ := sip.ParseURI(device.URIStr) // 通道地址
	device.addr = &sip.Address{URI: deviceURI}
	_serverDevices.addr.Params.Add("tag", sip.String{Str: utils.RandString(10)})
	hb := sip.NewHeaderBuilder().SetTo(device.addr).SetFrom(_serverDevices.addr).AddVia(&sip.ViaHop{
		Params: sip.NewParams().Add("branch", sip.String{Str: sip.GenerateBranch()}),
	}).SetContentType(&sip.ContentTypeSDP).SetMethod(sip.INVITE).SetContact(_serverDevices.addr)
	req := sip.NewRequest("", sip.INVITE, user.addr.URI, sip.DefaultSipVersion, hb.Build(), b)

	req.SetDestination(user.source) // 解析keepalive获取源地址
	req.AppendHeader(&sip.GenericHeader{HeaderName: "Subject", Contents: fmt.Sprintf("%s:%s,%s:%s", device.DeviceID, data.SSRC, _serverDevices.DeviceID, data.SSRC)})
	req.SetRecipient(device.addr.URI)
	logrus.Infof("invite req:\n%s\n", req.String())

	tx, err := srv.Request(req)
	if err != nil {
		logrus.Errorf("sipPlayPush request failed, channel id=%s, err=%s", device.DeviceID, err)
		return data, err
	}
	// response
	response, err := sipResponse(tx)
	if err != nil {
		logrus.Errorf("sipPlayPush response failed, channel id=%s, err=%s", device.DeviceID, err)
		return data, err
	}
	data.Resp = response
	logrus.Infof("invite resp:\n%s\n", response.String())

	// ACK
	ackReq := sip.NewRequestFromResponse(sip.ACK, response)
	ackReq.AppendHeader(&sip.GenericHeader{HeaderName: "User-Agent", Contents: USERAGENT})
	ackReq.AppendHeader(&sip.GenericHeader{HeaderName: "Max-Forwards", Contents: "70"})
	ackReq.AppendHeader(&sip.GenericHeader{HeaderName: "Content-Length", Contents: "0"})
	resTo, _ := response.To()
	ackReq.SetRecipient(resTo.Address)
	err = tx.Request(ackReq)
	// err = tx.Request(sip.NewRequestFromResponse(sip.ACK, response))
	if err != nil {
		logrus.Errorf("sipPlayPush ack failed, channel id=%s, err=%s", device.DeviceID, err)
		return data, err
	}
	logrus.Infof("invite ack\n")

	data.SSRC = ssrc2stream(data.SSRC)
	if len(data.SSRC) == 7 {
		data.SSRC = "0" + data.SSRC
	}
	data.streamType = m.StreamTypePush
	from, _ := response.From()
	to, _ := response.To()
	//callid, _ := response.CallID()
	toParams := map[string]string{}
	for k, v := range to.Params.Items() {
		toParams[k] = v.String()
	}
	fromParams := map[string]string{}
	for k, v := range from.Params.Items() {
		fromParams[k] = v.String()
	}
	// 成功后保存mongo，用来后续系统关闭推流使用
	//dbClient.Insert(streamTB, DeviceStream{
	//	T:          data.T,
	//	SSRC:       data.SSRC,
	//	DeviceID:   data.DeviceID, // 通道ID
	//	UserID:     data.UserID,   // 设备ID
	//	Status:     0,
	//	Time:       time.Now().Format("2006-01-02 15:04:05"),
	//	CallID:     (string)(*callid),
	//	Ttag:       toParams,
	//	Ftag:       fromParams,
	//	StreamName: data.streamName,
	//	StreamType: streamTypePush, //  pull 媒体服务器主动拉流，push 监控设备主动推流
	//})
	return data, err
}

// 向媒体服务器推流
func sipPlayPush(data *Streams, channel Channels, device Devices) (*Streams, error) {
	var (
		s sdp.Session
		b []byte
	)
	name := "Play"
	protocal := "TCP/RTP/AVP"
	if data.T == 1 { // 历史视频
		name = "Playback"
		protocal = "RTP/RTCP"
	} else if data.T == 2 { // 下载
		name = "Download"
	}

	video := sdp.Media{
		Description: sdp.MediaDescription{
			Type:     "video",
			Port:     _sysinfo.MediaServerRtpPort,
			Formats:  []string{"96", "98", "97"},
			Protocol: protocal,
		},
	}
	video.AddAttribute("recvonly")
	if data.T == 0 {
		video.AddAttribute("setup", "passive")
		video.AddAttribute("connection", "new")
	}
	video.AddAttribute("rtpmap", "96", "PS/90000")
	video.AddAttribute("rtpmap", "98", "H264/90000")
	video.AddAttribute("rtpmap", "97", "MPEG4/90000")

	// defining message
	msg := &sdp.Message{
		Origin: sdp.Origin{
			Username: _serverDevices.DeviceID, // 媒体服务器id
			Address:  _sysinfo.MediaServerRtpIP.String(),
		},
		Name: name,
		Connection: sdp.ConnectionData{
			IP:  _sysinfo.MediaServerRtpIP,
			TTL: 0,
		},
		Timing: []sdp.Timing{
			{
				Start: data.S,
				End:   data.E,
			},
		},
		Medias: []sdp.Media{video},
		SSRC:   data.ssrc,
	}
	if data.T == 1 {
		msg.URI = fmt.Sprintf("%s:0", channel.ChannelID)
	}

	// appending message to session
	s = msg.Append(s)
	// appending session to byte buffer
	b = s.AppendTo(b)
	uri, _ := sip.ParseURI(channel.URIStr)
	channel.addr = &sip.Address{URI: uri}
	_serverDevices.addr.Params.Add("tag", sip.String{Str: utils.RandString(20)})
	hb := sip.NewHeaderBuilder().SetTo(channel.addr).SetFrom(_serverDevices.addr).AddVia(&sip.ViaHop{
		Params: sip.NewParams().Add("branch", sip.String{Str: sip.GenerateBranch()}),
	}).SetContentType(&sip.ContentTypeSDP).SetMethod(sip.INVITE).SetContact(_serverDevices.addr)
	req := sip.NewRequest("", sip.INVITE, channel.addr.URI, sip.DefaultSipVersion, hb.Build(), b)
	req.SetDestination(device.source)
	req.AppendHeader(&sip.GenericHeader{HeaderName: "Subject", Contents: fmt.Sprintf("%s:%s,%s:%s", channel.ChannelID, data.StreamID, _serverDevices.DeviceID, data.StreamID)})
	req.SetRecipient(channel.addr.URI)
	tx, err := srv.Request(req)
	if err != nil {
		logrus.Warningln("sipPlayPush fail.id:", device.DeviceID, channel.ChannelID, "err:", err)
		return data, err
	}
	// response
	response, err := sipResponse(tx)
	if err != nil {
		logrus.Warningln("sipPlayPush response fail.id:", device.DeviceID, channel.ChannelID, "err:", err)
		return data, err
	}
	data.Resp = response
	// ACK
	tx.Request(sip.NewRequestFromResponse(sip.ACK, response))

	callid, _ := response.CallID()
	data.CallID = string(*callid)

	cseq, _ := response.CSeq()
	if cseq != nil {
		data.CseqNo = cseq.SeqNo
	}

	from, _ := response.From()
	to, _ := response.To()
	for k, v := range to.Params.Items() {
		data.Ttag[k] = v.String()
	}
	for k, v := range from.Params.Items() {
		data.Ftag[k] = v.String()
	}
	data.Status = 0

	return data, err
}

// sip 停止播放
func SipStopPlay(ssrc string) {
	zlmCloseStream(ssrc)
	data, ok := StreamList.Response.Load(ssrc)
	if !ok {
		return
	}
	play := data.(*Streams)
	if play.StreamType == m.StreamTypePush {
		// 推流，需要发送关闭请求
		resp := play.Resp
		u, ok := _activeDevices.Load(play.DeviceID)
		if !ok {
			return
		}
		user := u.(Devices)
		req := sip.NewRequestFromResponse(sip.BYE, resp)
		req.SetDestination(user.source)
		tx, err := srv.Request(req)
		if err != nil {
			logrus.Warningln("sipStopPlay bye fail.id:", play.DeviceID, play.ChannelID, "err:", err)
		}
		_, err = sipResponse(tx)
		if err != nil {
			logrus.Warnln("sipStopPlay response fail", err)
			play.Msg = err.Error()
		} else {
			play.Status = 1
			play.Stop = true
		}
		db.Save(db.DBClient, play)
	}
	StreamList.Response.Delete(ssrc)
	if play.T == 0 {
		StreamList.Succ.Delete(play.ChannelID)
	}
}

// sip 请求播放
func sipPlay(data playParams) (interface{}, error) {
	device := DeviceItem{}
	var err error

	if len(data.UserID) > 0 {
		//err = dbClient.Get(deviceTB, M{"deviceid": data.DeviceID, "pdid": data.UserID}, &device)
		err = db.GetQ(db.DBClient, new(Devices), db.M{"deviceid": data.DeviceID, "pdid": data.UserID}, nil)
	} else {
		//err = dbClient.Get(deviceTB, M{"deviceid": data.DeviceID}, &device)
		err = db.GetQ(db.DBClient, new(Devices), db.M{"deviceid": data.DeviceID}, nil)
	}
	if err != nil {
		//if err == ErrRecordNotFound {
		//	return nil, errors.New("未查询到注册相机")
		//}
		return nil, err
	}

	if time.Now().Unix()-device.Active > 30*60 {
		return nil, errors.New("相机已离线")
	}
	user, ok := _activeDevices.Get(device.PDID)
	if !ok {
		return nil, errors.New("NVR设备不存在")
	}
	data.UserID = user.DeviceID
	// 判断是否在拉流
	if succ, ok := _playList.devicesSucc.Load(device.PDID + device.DeviceID); ok && data.T == 0 {
		succ.(map[string]interface{})["up"] = data.up
		_playList.devicesSucc.Store(device.PDID+device.DeviceID, succ)
		return succ, nil
	}

	data, err = sipCasPlayPush(data, device, user)
	if err != nil {
		logrus.Error("sipPlay sipPlayPush failed, err=", err.Error())
		return nil, errors.New("pull stream from camera failed")
	}

	//s := zlmDis.GetServerItem(data.SSRC)
	//if s == nil {
	//	logrus.Error("sipPlay get zlmserver failed, ssrc=", data.SSRC)
	//	return nil, errors.New("zlmserver is not exist")
	//}
	succ := map[string]interface{}{
		"deviceid":  user.DeviceID,
		"ssrc":      data.SSRC,
		"http":      fmt.Sprintf("%s/rtp/%s/hls.m3u8", config.Media.HTTP, data.SSRC),
		"rtmp":      fmt.Sprintf("%s/rtp/%s", config.Media.RTMP, data.SSRC),
		"rtsp":      fmt.Sprintf("%s/rtp/%s", config.Media.RTSP, data.SSRC),
		"http-flv":  fmt.Sprintf("%s/rtp/%s.live.flv", config.Media.HTTP, data.SSRC),
		"streamNum": 0,
		"up":        data.up,
	}
	// data.UserID = user.DeviceID
	data.ext = time.Now().Unix() + 2*60 // 2分钟等待时间
	_playList.ssrcResponse.Store(data.SSRC, data)
	if data.T == 0 {
		_playList.devicesSucc.Store(device.PDID+device.DeviceID, succ)
	}
	return succ, nil
}

// sip 历史视频播放请求
func sipPlayback(did, ssrc, body string) (string, error) {
	device := DeviceItem{}
	//err := dbClient.Get(deviceTB, M{"deviceid": did}, &device)
	err := db.GetQ(db.DBClient, new(Devices), db.M{"deviceid": did}, nil)
	if err != nil {
		return "", err
	}

	d, ok := _playList.ssrcResponse.Load(ssrc)
	if !ok {
		logrus.Debugf("playback ssrc is not exist, ssrc:%s", ssrc)
		return "", errors.New("invite response is not exist")
	}
	play := d.(playParams)

	req := sip.NewRequestFromResponse(sip.INFO, play.Resp)
	deviceURI, _ := sip.ParseURI(device.URIStr) // 通道地址
	req.SetRecipient(deviceURI)
	req.AppendHeader(&sip.GenericHeader{HeaderName: "Max-Forwards", Contents: "70"})
	req.AppendHeader(&sip.GenericHeader{HeaderName: "Content-Type", Contents: string(sip.ContentTypeRtsp)})
	req.AppendHeader(&sip.GenericHeader{HeaderName: "User-Agent", Contents: USERAGENT})
	req.SetBody([]byte(body), true)

	tx, err := srv.Request(req)
	if err != nil {
		return "", err
	}
	logrus.Infof("info req:\n%s", req.String())
	// response
	res, err := sipResponse(tx)
	if err != nil {
		return "", err
	}
	logrus.Debugf("info res:\n%s", res.String())
	return string(res.Body()), nil
}
