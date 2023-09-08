package sipapi

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	sip "github.com/panjjo/gosip/sip/s"
	"github.com/panjjo/gosip/utils"
	"github.com/sirupsen/logrus"

	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// STREAMTB 级联播放流表名
	STREAMTB = "rtpstreams"
)

type playInfo struct {
	chanID string
	stream string
	seq    uint
	host   string
	port   int
}

// playParams 播放请求参数
type playParams struct {
	// 0  直播 1 历史 2 下载
	T int
	//  开始结束时间，只有t=1 时有效
	S, E       time.Time
	SSRC       string
	Resp       *sip.Response
	DeviceID   string
	UserID     string
	ext        int64  // 推流等待的过期时间，用于判断是否请求成功但推流失败。超过还未接收到推流定义为失败，重新请求推流或者关闭此ssrc
	stream     bool   // 是否完成推流，用于web_hook 出现stream=false时等待推流，出现stream_not_found 且 stream=true表示推流过但已关闭。释放ssrc。
	streamType string // pull 媒体服务器主动拉流，push 监控设备主动推流
	streamName string // 流名称
	up         bool   // 上级平台是否观看视频
	dlspeed    string // 下载速度
	cstream    string // 码流
	rtspseq    int    // 命令序列
	// fps        int    // 帧率
	//track registry.MediaListTrack
}

// ChannelStream channel流信息
type ChannelStream struct {
	// ChannelID 通道ID
	ChannelID string `json:"cid" bson:"cid"`
	// Host 收流地址
	Host string `json:"host" bson:"host"`
	// Region 设备域
	Port int `json:"port" bson:"port"`
	// CallID INVITE唯一ID
	CallID     string `json:"callid" bson:"callid"`
	Stream     string `json:"stream" bson:"stream"` // 当前流名称
	SSRC       int    `json:"ssrc" bson:"ssrc"`
	Status     int    `json:"status" bson:"status"`
	Stop       bool   `json:"stop" bson:"stop"`
	Time       string `json:"time" bson:"time"`
	StreamKey  string `json:"key" bson:"key"`
	LocPort    int    `json:"locPort" bson:"locPort"`
	PlayT      int    `json:"playt" bson:"playt"` // 播放类型，0-实时，1-历史
	TcpPort    int    `json:"tcpPort" bson:"tcpPort"`
	IsActive   bool   `json:"isActive" bson:"isActive"`
	StreamType string `json:"stype" bson:"stype"` // 流类型，0-主码流，1-子
}

var (
	ssrcMap   *sync.Map // 每个通道视频观看路数
	callIDMap *sync.Map // 每个视频播放请求信息，使用唯一callid
	// nvrMap    *sync.Map // NVR设备IP对应播放路数，超过指定路数拒绝后续连接
	rtpSeq  uint
	rtpLock *sync.Mutex
	// transCmd    *exec.Cmd // 转码子码流命令
	playbackMap *sync.Map // 历史回放记录
	ffmpegMap   *sync.Map // 调用ffmpeg命令记录
)

func createTryingResponse(req *sip.Request) *sip.Response {

	resp := sip.NewResponseFromRequest("", req, http.StatusContinue, "Trying", []byte(""))
	resp.AppendHeader(&sip.GenericHeader{HeaderName: "User-Agent", Contents: USERAGENT})
	resp.AppendHeader(&sip.GenericHeader{HeaderName: "Content-Length", Contents: "0"})

	return resp
}

func inviteStreamFromDevice(d playParams) (string, error) {
	var ssrc string
	var num int
	res, err := sipPlay(d)
	if err != nil {
		logrus.Errorf("inviteStreamFromDevice sipPlay failed, err=%s", err)
		return "", err
	}
	logrus.Infof("inviteStreamFromDevice sipPlay result:%+v", res)

	// 调用zlm api
	num = res.(map[string]interface{})["streamNum"].(int)
	ssrc = res.(map[string]interface{})["ssrc"].(string)
	// 下级平台没有取视频
	if num == 0 && d.T != 2 {
		//_, err := requestZLMAPI(ssrc)
		//if err != nil {
		//	return "", err
		//}
	}
	return ssrc, nil
}

func subStreamAddProxy(ssrc string) (string, error) {
	//stream := "stream-" + ssrc
	//rtmp := config.Media.RTMP + "/" + ssrc
	//body, err := zlmAddStreamProxy(rtmp, stream)
	//if err != nil {
	//	logrus.Errorf("subStreamAddProxy zlm failed, err:%s", err)
	//	return "", err
	//}
	var m map[string]interface{}
	//err = utils.JSONDecode(body, &m)
	//if err != nil {
	//	logrus.Errorf("subStreamAddProxy decode zlm json failed, body:%s, err:%s", string(body), err)
	//	return "", err
	//}
	code := int(m["code"].(float64))
	if code != 0 {
		msg := m["msg"].(string)
		logrus.Errorf("subStreamAddProxy zlm error, response code:%d, msg:%s", code, msg)
		//return "", err
	}
	m2 := m["data"].(map[string]interface{})
	key := m2["key"].(string)
	return key, nil
}

func sendRtpServer(isUDP int, c *ChannelStream, d playParams) error {

	data, ok := _playList.ssrcResponse.Load(c.Stream)
	if !ok {
		logrus.Errorf("sendRtpServer could not find play list, ssrc=%s", c.Stream)
		return errors.New("ssrc is not exist")
	}
	p := data.(playParams)
	var num int
	succ, ok := _playList.devicesSucc.Load(p.UserID + p.DeviceID)
	if ok {
		num = succ.(map[string]interface{})["streamNum"].(int)
	}

	streamName := c.Stream

	err := sendNewRtpServer(isUDP, c, streamName)
	if err != nil {
		if num == 0 {
			//sipStopPlay(c.Stream)
		}
		logrus.Errorf("sendRtpServer newrtp failed, deviceid=%s, callid=%s, err=%s", c.ChannelID, c.CallID, err)
		if d.T == 1 {
			playbackMap.Delete(c.CallID)
		}
		return err
	}
	// 某些情况下sip bye信令会提前发送，
	_, ok = hisCallMap.Load(c.CallID)
	if !ok {
		closeChannelStream(c.ChannelID, c.CallID)
		return nil
	}
	// 只记录实时视频
	if c.PlayT == 0 {
		data, ok := ssrcMap.Load(c.ChannelID)
		if ok {
			num := data.(int)
			num++
			ssrcMap.Store(c.ChannelID, num)
		} else {
			ssrcMap.Store(c.ChannelID, 1)
		}
	}

	return nil
}

func sendRtspRtpServer(isUDP int, cs *ChannelStream) error {
	nvrData, err := getNvrData(cs.ChannelID)
	if err != nil {
		logrus.Errorf("sendRtspRtpServer getNvrData failed, id=%s, err=%s", cs.ChannelID, err)
		return err
	}

	var (
		rtspURL string
		stype   int
	)
	switch nvrData.DevType {
	case "HK":
		stype = 1
		if cs.StreamType == "1" {
			stype = 2
		}
		rtspURL = fmt.Sprintf("rtsp://%s:%s@%s:%s/Streaming/Channels/%s0%d",
			nvrData.NVRUser, nvrData.NVRPwd, nvrData.NVRIP, nvrData.RtspPort, nvrData.ChannelCode, stype)
	case "DH":
		stype = 0
		if cs.StreamType == "1" {
			stype = 1
		}
		rtspURL = fmt.Sprintf("rtsp://%s:%s@%s:%s/cam/realmonitor?channel=%s&subtype=%d",
			nvrData.NVRUser, nvrData.NVRPwd, nvrData.NVRIP, nvrData.RtspPort, nvrData.ChannelCode, stype)
	case "ZY":
		// rtsp://admin:zhht2016@10.100.32.140:8554/live/channel/1?subtype=2
		rtspURL = fmt.Sprintf("rtsp://%s:%s@%s:%s/live/channel/%s?subtype=2",
			nvrData.NVRUser, nvrData.NVRPwd, nvrData.NVRIP, nvrData.RtspPort, nvrData.ChannelCode)
	default:
		return errors.New("device type is invalid")
	}
	logrus.Debug("rtsp url=", rtspURL)
	//escapeUrl := url.QueryEscape(rtspURL)

	stream := fmt.Sprintf("stream%s", cs.ChannelID)
	//body, err := zlmAddStreamProxy(escapeUrl, stream)
	//if err != nil {
	//	logrus.Error("sendRtspRtpServer addStreamProxy failed, err=", err)
	//	return err
	//}
	var m map[string]interface{}
	//err = utils.JSONDecode(body, &m)
	//if err != nil {
	//	logrus.Errorf("sendRtspRtpServer addStreamProxy json error, body=%s, err=%s", string(body), err)
	//	return err
	//}
	code := int(m["code"].(float64))
	if code != 0 {
		msg := m["msg"].(string)
		logrus.Errorf("sendRtspRtpServer addStreamProxy error, code=%d, msg=%s", code, msg)
		return err
	}
	m2 := m["data"].(map[string]interface{})
	key := m2["key"].(string)
	cs.StreamKey = key
	cs.Stream = stream
	// 推送国标视频流
	waitPushStream(stream)
	return nil
}

func waitPushStream(ssrc string) {
	//start := time.Now()
	for {
		//d := time.Since(start)
		//if time.Duration(d) > time.Duration(config.StreamTime)*time.Second {
		//	logrus.Infof("waitPushStream exceed %d seconds, timeout", config.StreamTime)
		//	break
		//}
		//_, ok := rtpStreamMap.Load(ssrc)
		//if ok {
		//	logrus.Infof("waitPushStream get stream, ssrc=%s", ssrc)
		//	break
		//}
		time.Sleep(100 * time.Millisecond)
	}
	//rtpStreamMap.Delete(ssrc)
}

func canPlay(chanID string) (bool, error) {
	//d, ok := ssrcMap.Load(chanID)
	//if ok {
	//	//num := d.(int)
	//	//if num >= config.PlayCtl.DownNum {
	//	//	return false, nil
	//	//}
	//}
	return true, nil
}

func sendNewRtpServer(isUDP int, c *ChannelStream, ssrc string) error {
	rtpLock.Lock()
	rtpSeq++
	if rtpSeq > 100000 {
		rtpSeq = 1
	}
	tmpSeq := rtpSeq
	rtpLock.Unlock()

	// send rtp data
	var body []byte
	var err error
	//if c.IsActive {
	//	body, err = zlmStartSendRtpPassive(tmpSeq, ssrc, c.TcpPort)
	//} else {
	//	body, err = zlmStartSendRtp(tmpSeq, ssrc, isUDP, c)
	//}
	if err != nil {
		logrus.Errorf("start send new rtp server failed, err=%s", err)
		return err
	}
	var m map[string]interface{}
	err = utils.JSONDecode(body, &m)
	if err != nil {
		logrus.Errorf("decode send new rtp json failed, body=%s, err=%s", string(body), err)
		return err
	}
	code := int(m["code"].(float64))
	if code != 0 {
		msg := m["msg"].(string)
		logrus.Errorf("sendNewRtpServer response error, code=%d, msg=%s", code, msg)
		return errors.New(msg)
	}
	//locPort := int(m["local_port"].(float64))

	p := &playInfo{}
	p.chanID = c.ChannelID
	p.host = c.Host
	p.port = c.Port
	p.seq = tmpSeq
	p.stream = ssrc
	callIDMap.Store(c.CallID, p)

	// 成功后保存mongo，用来后续系统关闭推流使用
	//dbClient.Insert(STREAMTB, ChannelStream{
	//	ChannelID: c.ChannelID,
	//	Host:      c.Host,
	//	Port:      c.Port,
	//	CallID:    c.CallID,
	//	Stream:    ssrc,
	//	SSRC:      int(tmpSeq),
	//	Status:    0,
	//	Stop:      false,
	//	Time:      time.Now().Format("2006-01-02 15:04:05"),
	//	StreamKey: c.StreamKey,
	//	LocPort:   locPort,
	//	PlayT:     c.PlayT,
	//	IsActive:  c.IsActive,
	//})
	logrus.Infof("start to send gb stream, stream=%s, ssrc=%d, ip=%s, port=%d", ssrc, tmpSeq, c.Host, c.Port)
	return nil
}

func stopSendRtp(stream string, ssrc int, key string) error {
	if ssrc > 0 {
		//err := zlmStopSendRtp(stream, ssrc)
		//if err != nil {
		//	logrus.Errorf("stopSendRtp failed, err=%s", err)
		//	return err
		//}
		//dbClient.Update(STREAMTB, M{"stream": stream, "ssrc": ssrc, "stop": false}, M{"$set": M{"status": 1, "stop": true}})
	}
	if len(key) > 0 {
		// 停止拉流代理
		//_ = zlmDelStreamProxy(key)
		s := stream
		ss := strings.Split(stream, "-")
		if len(ss) == 2 {
			s = ss[1]
		}
		v, ok := ffmpegMap.Load(s)
		if ok {
			cmd := v.(*exec.Cmd)
			err := stopSubStream(cmd)
			if err != nil {
				logrus.Errorf("stopSubStream failed, err:%s", err)
			}
			ffmpegMap.Delete(s)
		}
	}
	return nil
}

func closeChannelStream(deviceID, callID string) {
	cstm := &ChannelStream{}
	//dbClient.Get(STREAMTB, M{"callid": callID}, cstm)
	playbackMap.Delete(callID)
	if cstm.ChannelID == "" || cstm.Stream == "" {
		logrus.Debugf("don't start send rtp, did:%s, callid:%s", deviceID, callID)
		return
	}

	// 判断是否上下级同时观看，按需断流
	var (
		play  playParams
		close bool
	)
	ssrc := cstm.Stream
	ss := strings.Split(cstm.Stream, "-")
	if len(ss) == 2 {
		ssrc = ss[1]
	}
	if data, ok := _playList.ssrcResponse.Load(ssrc); ok {
		play = data.(playParams)
		if succ, ok := _playList.devicesSucc.Load(play.UserID + play.DeviceID); ok {
			num := succ.(map[string]interface{})["streamNum"].(int)
			if num == 0 {
				close = true
			}
		}
	}
	// end

	data, ok := callIDMap.Load(callID)
	if !ok {
		logrus.Warnf("closeChannelStream could not find callid, id=%s", callID)

		stopSendRtp(cstm.Stream, cstm.SSRC, cstm.StreamKey)
		//sipStopPlay(ssrc)
		logrus.Infof("closeChannelStream stop sending and delete stream by db, stream=%s, ssrc=%d", cstm.Stream, cstm.SSRC)
		return
	}

	ssrcData, ok := ssrcMap.Load(deviceID)
	if !ok { // 历史视频播放
		stopSendRtp(cstm.Stream, cstm.SSRC, cstm.StreamKey)
		//sipStopPlay(ssrc)
		callIDMap.Delete(callID)
		logrus.Warnf("closeChannelStream finished, could not find deviceid, id=%s", deviceID)
		return
	}
	num := ssrcData.(int)
	p := data.(*playInfo)
	if num > 1 {
		stopSendRtp(p.stream, int(p.seq), cstm.StreamKey)

		callIDMap.Delete(callID)
		num--
		ssrcMap.Store(deviceID, num)
		logrus.Infof("closeChannelStream stop sending one rtp, stream=%s, ssrc=%d, num=%d", p.stream, p.seq, num)
		return
	}

	stopSendRtp(p.stream, int(p.seq), cstm.StreamKey)
	//rtpStreamMap.Delete(ssrc)
	callIDMap.Delete(callID)
	ssrcMap.Delete(deviceID)
	logrus.Infof("closeChannelStream stop sending and delete stream, stream=%s, ssrc=%d", p.stream, p.seq)

	if succ, ok := _playList.devicesSucc.Load(play.UserID + play.DeviceID); ok {
		succ.(map[string]interface{})["up"] = false
		_playList.devicesSucc.Store(play.UserID+play.DeviceID, succ)
	}
	if close {
		//sipStopPlay(ssrc)
		logrus.Infof("closeChannelStream close all streams, stream=%s", p.stream)
	}
}

func checkRtpInfo(ssrc string) error {
	//body, err := zlmGetRtpInfo(ssrc)
	//if err != nil {
	//	return err
	//}

	var m map[string]interface{}
	//err = utils.JSONDecode(body, &m)
	//if err != nil {
	//	logrus.Errorf("decode rtp info json failed, body=%s, err=%s", string(body), err)
	//	return err
	//}
	code := int(m["code"].(float64))
	if code != 0 {
		logrus.Errorf("checkRtpInfo response error, code=%d", code)
		return errors.New("get rtp info failed")
	}
	exist := m["exist"].(bool)
	if !exist {
		return errors.New("rtp info is not exist")
	}
	logrus.Infof("checkRtpInfo finished. ssrc(%s) is exist", ssrc)
	return nil
}

func checkMediaList(ssrc string) error {
	//body, err := zlmGetMediaList(ssrc)
	//if err != nil {
	//	return err
	//}

	var m map[string]interface{}
	//err = utils.JSONDecode(body, &m)
	//if err != nil {
	//	logrus.Errorf("checkMediaList decode json failed, body=%s, err=%s", string(body), err)
	//	return err
	//}
	code := int(m["code"].(float64))
	if code != 0 {
		logrus.Errorf("checkMediaList response error, code=%d", code)
		return errors.New("get media list failed")
	}
	exist := false
	data := m["data"].([]interface{})
	for _, v := range data {
		dd := v.(map[string]interface{})
		schema := dd["schema"].(string)
		totalRdCnt := int(dd["totalReaderCount"].(float64))
		if schema == "rtsp" && totalRdCnt > 0 {
			exist = true
			break
		}
	}
	if !exist {
		logrus.Infof("checkMediaList stream is not exist, ssrc=%s", ssrc)
		return errors.New("stream is not exist")
	}
	logrus.Infof("checkMediaList finished. ssrc(%s) is exist", ssrc)
	return nil
}

// transSubStream2 转码子码流
func transSubStream2(ssrc string) error {
	//data, ok := _playList.ssrcResponse.Load(ssrc)
	//if !ok {
	//	logrus.Errorf("transSubStream2 could not find play list, ssrc=%s", ssrc)
	//	return errors.New("ssrc is not exist")
	//}
	//p := data.(playParams)
	//var rtsp string
	//succ, ok := _playList.devicesSucc.Load(p.UserID + p.DeviceID)
	//if !ok {
	//	logrus.Errorf("transSubStream2 could not find rtsp url, devid=%s", p.DeviceID)
	//	return errors.New("rtsp is not exist")
	//}
	//rtsp = succ.(map[string]interface{})["rtmp"].(string)

	//rtmp := config.Media.SSRtmp + "/" + ssrc
	//logrus.Debugf("transcode sub stream, ssrc:%s, rtsp:%s, rtmp:%s", ssrc, rtsp, rtmp)
	//body, err := zlmAddFFmpegSource(rtsp, rtmp)
	//if err != nil {
	//	logrus.Errorf("transSubStream2 zlm failed, err:%s", err)
	//	return err
	//}
	var m map[string]interface{}
	//err = utils.JSONDecode(body, &m)
	//if err != nil {
	//	logrus.Errorf("transSubStream2 decode zlm json failed, body:%s, err:%s", string(body), err)
	//	return err
	//}
	code := int(m["code"].(float64))
	if code != 0 {
		msg := m["msg"].(string)
		logrus.Errorf("transSubStream2 zlm error, response code:%d, msg:%s", code, msg)
		//return err
	}
	m2 := m["data"].(map[string]interface{})
	key := m2["key"].(string)
	ffmpegMap.Store(ssrc, key)
	logrus.Debugf("transcode sub stream running, ssrc:%s", ssrc)

	return nil
}

func stopSubStream2(ssrc string) error {
	//v, ok := ffmpegMap.Load(ssrc)
	//if !ok {
	//	return errors.New("ffmpeg key is not exist")
	//}
	//err := zlmDelFFmpegSource(v.(string))
	ffmpegMap.Delete(ssrc)
	logrus.Debugf("stop sub stream, ssrc:%s", ssrc)
	//return err
	return nil
}

func transSubStream(ssrc string) (*exec.Cmd, error) {
	data, ok := _playList.ssrcResponse.Load(ssrc)
	if !ok {
		logrus.Errorf("transSubStream could not find play list, ssrc=%s", ssrc)
		return nil, errors.New("ssrc is not exist")
	}
	p := data.(playParams)
	//var rtsp string
	//succ, ok := _playList.devicesSucc.Load(p.UserID + p.DeviceID)
	if !ok {
		logrus.Errorf("transSubStream could not find stream info, devid=%s", p.DeviceID)
		return nil, errors.New("stream is not exist")
	}
	//rtsp = succ.(map[string]interface{})["rtmp"].(string)

	// 由于使用全局变量，先杀死前面使用的，在启动新进程
	// _ = stopSubStream(transCmd)
	// 启动新进程
	//rtmp := config.Media.SSRtmp + "/" + ssrc
	//param := fmt.Sprintf(config.FFmpegCmd, rtsp, rtmp)
	//cmd := exec.Command("/bin/bash", "-c", param)
	//cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	//logrus.Debugf("transSubStream exec cmd:%s, args:%+v", param, cmd.Args)
	//
	//go func() {
	//	var out bytes.Buffer
	//	cmd.Stdout = &out
	//	cmd.Stderr = os.Stderr
	//	_ = cmd.Run()
	//}()
	//return cmd, nil
	return nil, nil
}

func stopSubStream(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return errors.New("process is not exist")
	}
	logrus.Info("stopSubStream pid:", cmd.Process.Pid)

	//err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	//if err != nil {
	//	return err
	//}

	return nil
}

// parseMansRtsp return time and scale
func parseMansRtsp(data []byte) (int, float32) {
	reader := bufio.NewReader(bytes.NewReader(data))
	t := -1
	scale := 1.0
	for {
		str, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		if len(str) >= 2 {
			str = str[:len(str)-2]
		}
		if len(str) < 7 {
			continue
		}
		head := str[:5]
		switch head {
		case "Range":
			// Range: npt=3112-
			arr := strings.Split(str, "=")
			if len(arr) != 2 {
				continue
			}
			i := strings.Index(arr[1], "-")
			if i == -1 {
				continue
			}
			if arr[1][:i] != "now" {
				t, _ = strconv.Atoi(arr[1][:i])
			} else {
				t = 0
			}
		case "Scale":
			ss := strings.Trim(str[6:], " ")
			scale, _ = strconv.ParseFloat(ss, 32)
		}
	}
	return t, float32(scale)
}
