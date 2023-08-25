package sipapi

import (
	"errors"
	"fmt"
	"github.com/panjjo/gosip/m"
	"net/url"
	"strconv"

	"github.com/panjjo/gosip/utils"
	"github.com/sirupsen/logrus"
)

type zlmGetMediaListReq struct {
	vhost    string
	schema   string
	streamID string
	app      string
}
type zlmGetMediaListResp struct {
	Code int                       `json:"code"`
	Data []zlmGetMediaListDataResp `json:"data"`
}
type zlmGetMediaListDataResp struct {
	App        string                  `json:"app"`
	Stream     string                  `json:"stream"`
	Schema     string                  `json:"schema"`
	OriginType int                     `json:"originType"`
	Tracks     []zlmGetMediaListTracks `json:"tracks"`
}
type zlmGetMediaListTracks struct {
	Type    int `json:"codec_type"`
	CodecID int `json:"codec_id"`
	Height  int `json:"height"`
	Width   int `json:"width"`
	FPS     int `json:"fps"`
}

// zlm 获取流列表信息
func zlmGetMediaList(req zlmGetMediaListReq) zlmGetMediaListResp {
	res := zlmGetMediaListResp{}
	reqStr := "/index/api/getMediaList?secret=" + config.Media.Secret
	if req.streamID != "" {
		reqStr += "&stream=" + req.streamID
	}
	if req.app != "" {
		reqStr += "&app=" + req.app
	}
	if req.schema != "" {
		reqStr += "&schema=" + req.schema
	}
	if req.vhost != "" {
		reqStr += "&vhost=" + req.vhost
	}
	body, err := utils.GetRequest(config.Media.RESTFUL + reqStr)
	if err != nil {
		logrus.Errorln("get stream mediaList fail,", err)
		return res
	}
	if err = utils.JSONDecode(body, &res); err != nil {
		logrus.Errorln("get stream mediaList fail,", err)
		return res
	}
	logrus.Traceln("zlmGetMediaList ", string(body), req.streamID)
	return res
}

var zlmDeviceVFMap = map[int]string{
	0: "H264",
	1: "H265",
	2: "ACC",
	3: "G711A",
	4: "G711U",
}

func transZLMDeviceVF(t int) string {
	if v, ok := zlmDeviceVFMap[t]; ok {
		return v
	}
	return "undefind"
}

type rtpInfo struct {
	Code  int  `json:"code"`
	Exist bool `json:"exist"`
}

// 获取流在zlm上的信息
func zlmGetMediaInfo(ssrc string) rtpInfo {
	res := rtpInfo{}
	body, err := utils.GetRequest(config.Media.RESTFUL + "/index/api/getRtpInfo?secret=" + config.Media.Secret + "&stream_id=" + ssrc)
	if err != nil {
		logrus.Errorln("get stream rtpInfo fail,", err)
		return res
	}
	if err = utils.JSONDecode(body, &res); err != nil {
		logrus.Errorln("get stream rtpInfo fail,", err)
		return res
	}
	return res
}

// zlm 关闭流
func zlmCloseStream(ssrc string) {
	utils.GetRequest(config.Media.RESTFUL + "/index/api/close_streams?secret=" + config.Media.Secret + "&stream=" + ssrc)
}

// zlm 开始录制视频流
func zlmStartRecord(values url.Values) error {
	body, err := utils.GetRequest(config.Media.RESTFUL + "/index/api/startRecord?" + values.Encode())
	if err != nil {
		return err
	}
	tmp := map[string]interface{}{}
	err = utils.JSONDecode(body, &tmp)
	if err != nil {
		return err
	}
	if code, ok := tmp["code"]; !ok || fmt.Sprint(code) != "0" {
		return utils.NewError(nil, tmp)
	}
	return nil
}

// zlm 停止录制
func zlmStopRecord(values url.Values) error {
	body, err := utils.GetRequest(config.Media.RESTFUL + "/index/api/stopRecord?" + values.Encode())
	if err != nil {
		return err
	}
	tmp := map[string]interface{}{}
	err = utils.JSONDecode(body, &tmp)
	if err != nil {
		return err
	}
	if code, ok := tmp["code"]; !ok || fmt.Sprint(code) != "0" {
		return utils.NewError(nil, tmp)
	}
	return nil
}

// 功能：通过fork FFmpeg进程的方式拉流代理，支持任意协议
//
// 范例：http://127.0.0.1/index/api/addFFmpegSource?src_url=http://live.hkstv.hk.lxdns.com/live/hks2/playlist.m3u8&dst_url=rtmp://127.0.0.1/live/hks2&timeout_ms=10000&ffmpeg_cmd_key=ffmpeg.cmd
func zlmAddFFmpegSource(srcUrl, dstUrl string, timeout_ms int64, enable_hls, enable_mp4 bool, ffmpeg_cmd_key string) (string, error) {
	params := map[string]string{}
	params["srcUrl"] = srcUrl
	params["dstUrl"] = dstUrl
	params["timeout_ms"] = string(timeout_ms)
	params["enable_hls"] = strconv.FormatBool(enable_hls)
	params["enable_mp4"] = strconv.FormatBool(enable_mp4)
	params["ffmpeg_cmd_key"] = ffmpeg_cmd_key
	var str string
	for k, v := range params {
		str += "&" + k + "=" + v
	}
	body, err := utils.GetRequest(m.MConfig.Media.RESTFUL + "/index/api/addFFmpegSource?secret=" + m.MConfig.Media.Secret + str)
	if err != nil {
		logrus.Errorln("set server config failed, err=", err)
		return "", err
	}
	resp := map[string]interface{}{}
	err = utils.JSONDecode(body, &resp)
	if err != nil {
		logrus.Errorln("set config: json decode failed, err=", err)
		return "", err
	}
	if code, ok := resp["code"]; !ok || fmt.Sprint(code) != "0" {
		logrus.Errorln("set config: response error")
		return "", errors.New("set config: response error")
	}
	logrus.Infof("invoke interface zlm-addFFmpegSource response:\n%s", string(body))
	return string(body), nil
}

// 功能：动态添加rtsp/rtmp/hls/http-ts/http-flv拉流代理(只支持H264/H265/aac/G711/opus负载)
// 范例：http://127.0.0.1/index/api/addStreamProxy?vhost=__defaultVhost__&app=proxy&stream=0&url=rtmp://live.hkstv.hk.lxdns.com/live/hks2
func zlmAddStreamProxy(vhost, app, stream, url string,
	retry_count, rtp_type, timeout_sec int,
	enable_hls, enable_hls_fmp4, enable_mp4, enable_rtsp, enable_rtmp, enable_ts, enable_fmp4, hls_demand, rtsp_demand, rtmp_demand, ts_demand, fmp4_demand, enable_audio, add_mute_audio bool,
	mp4_save_path string, mp4_max_second int, mp4_as_player bool, hls_save_path string, modify_stamp int, auto_close bool) (string, error) {
	params := map[string]string{}
	params["vhost"] = vhost
	params["app"] = app
	params["stream"] = stream
	params["url"] = url
	params["retry_count"] = string(retry_count)
	params["rtp_type"] = string(rtp_type)
	params["timeout_sec"] = string(timeout_sec)
	params["enable_hls"] = strconv.FormatBool(enable_hls)
	params["enable_hls_fmp4"] = strconv.FormatBool(enable_hls_fmp4)
	params["enable_mp4"] = strconv.FormatBool(enable_mp4)
	params["enable_rtsp"] = strconv.FormatBool(enable_rtsp)
	params["enable_rtmp"] = strconv.FormatBool(enable_rtmp)
	params["enable_ts"] = strconv.FormatBool(enable_ts)
	params["enable_fmp4"] = strconv.FormatBool(enable_fmp4)
	params["hls_demand"] = strconv.FormatBool(hls_demand)
	params["rtsp_demand"] = strconv.FormatBool(rtsp_demand)
	params["rtmp_demand"] = strconv.FormatBool(rtmp_demand)
	params["ts_demand"] = strconv.FormatBool(ts_demand)
	params["fmp4_demand"] = strconv.FormatBool(fmp4_demand)
	params["enable_audio"] = strconv.FormatBool(enable_audio)
	params["add_mute_audio"] = strconv.FormatBool(add_mute_audio)
	params["mp4_save_path"] = mp4_save_path
	params["mp4_max_second"] = string(mp4_max_second)
	params["mp4_as_player"] = strconv.FormatBool(mp4_as_player)
	params["hls_save_path"] = hls_save_path
	params["modify_stamp"] = string(modify_stamp)
	params["auto_close"] = strconv.FormatBool(auto_close)
	var str string
	for k, v := range params {
		str += "&" + k + "=" + v
	}
	body, err := utils.GetRequest(m.MConfig.Media.RESTFUL + "/index/api/addStreamProxy?secret=" + m.MConfig.Media.Secret)
	if err != nil {
		logrus.Errorln("set server config failed, err=", err)
		return "", err
	}
	resp := map[string]interface{}{}
	err = utils.JSONDecode(body, &resp)
	if err != nil {
		logrus.Errorln("set config: json decode failed, err=", err)
		return "", err
	}
	if code, ok := resp["code"]; !ok || fmt.Sprint(code) != "0" {
		logrus.Errorln("set config: response error")
		return "", errors.New("set config: response error")
	}
	logrus.Infof("invoke interface zlm-addStreamProxy response:\n%s", string(body))
	return string(body), nil
}

// 功能：添加rtsp/rtmp主动推流(把本服务器的直播流推送到其他服务器去)
//
// 范例：http://127.0.0.1/index/api/addStreamPusherProxy?vhost=__defaultVhost__&app=proxy&stream=test&dst_url=rtmp://127.0.0.1/live/test2
func zlmAddStreamPusherProxy(vhost, schema, app, stream, dst_url string, retry_count, rtp_type int, timeout_sec float64) (string, error) {
	params := map[string]string{}
	params["vhost"] = vhost
	params["schema"] = schema
	params["app"] = app
	params["stream"] = stream
	params["dst_url"] = dst_url
	params["retry_count"] = string(retry_count)
	params["rtp_type"] = string(rtp_type)
	params["timeout_sec"] = strconv.FormatFloat(timeout_sec, 'f', 2, 32)
	var str string
	for k, v := range params {
		str += "&" + k + "=" + v
	}
	body, err := utils.GetRequest(m.MConfig.Media.RESTFUL + "/index/api/addStreamPusherProxy?secret=" + m.MConfig.Media.Secret + str)
	if err != nil {
		logrus.Errorln("set server config failed, err=", err)
		return "", err
	}
	resp := map[string]interface{}{}
	err = utils.JSONDecode(body, &resp)
	if err != nil {
		logrus.Errorln("set config: json decode failed, err=", err)
		return "", err
	}
	if code, ok := resp["code"]; !ok || fmt.Sprint(code) != "0" {
		logrus.Errorln("set config: response error")
		return "", errors.New("set config: response error")
	}
	logrus.Infof("invoke interface zlm-addStreamPusherProxy response:\n%s", string(body))
	return string(body), nil
}

// 功能：创建GB28181 RTP接收端口，如果该端口接收数据超时，则会自动被回收(不用调用closeRtpServer接口)
//
// 范例：http://127.0.0.1/index/api/openRtpServer?port=0&tcp_mode=1&stream_id=test
func zlmCloseRtpServer(port, tcp_mode int, stream_id string) (string, error) {
	params := map[string]string{}
	params["port"] = string(port)
	params["tcp_mode"] = string(tcp_mode)
	params["stream_id"] = stream_id

	var str string
	for k, v := range params {
		str += "&" + k + "=" + v
	}
	body, err := utils.GetRequest(m.MConfig.Media.RESTFUL + "/index/api/closeRtpServer?secret=" + m.MConfig.Media.Secret + str)
	if err != nil {
		logrus.Errorln("set server config failed, err=", err)
		return "", err
	}
	resp := map[string]interface{}{}
	err = utils.JSONDecode(body, &resp)
	if err != nil {
		logrus.Errorln("set config: json decode failed, err=", err)
		return "", err
	}
	if code, ok := resp["code"]; !ok || fmt.Sprint(code) != "0" {
		logrus.Errorln("set config: response error")
		return "", errors.New("set config: response error")
	}
	logrus.Infof("invoke interface zlm-closeRtpServer response:\n%s", string(body))
	return string(body), nil
}

// 功能：关闭流(目前所有类型的流都支持关闭)
//
// 范例：http://127.0.0.1/index/api/close_streams?schema=rtmp&vhost=__defaultVhost__&app=live&stream=0&force=1
func zlmCloseStreams(schema, vhost, app, stream string, force bool) (string, error) {
	params := map[string]string{}
	params["schema"] = schema
	params["vhost"] = vhost
	params["app"] = app
	params["stream"] = stream
	params["force"] = strconv.FormatBool(force)
	var str string
	for k, v := range params {
		str += "&" + k + "=" + v
	}
	body, err := utils.GetRequest(m.MConfig.Media.RESTFUL + "/index/api/close_streams?secret=" + m.MConfig.Media.Secret + str)
	if err != nil {
		logrus.Errorln("set server config failed, err=", err)
		return "", err
	}
	resp := map[string]interface{}{}
	err = utils.JSONDecode(body, &resp)
	if err != nil {
		logrus.Errorln("set config: json decode failed, err=", err)
		return "", err
	}
	if code, ok := resp["code"]; !ok || fmt.Sprint(code) != "0" {
		logrus.Errorln("set config: response error")
		return "", errors.New("set config: response error")
	}
	logrus.Infof("invoke interface zlm-close_streams response:\n%s", string(body))
	return string(body), nil
}

// 功能：关闭ffmpeg拉流代理
//
// 范例：http://127.0.0.1/index/api/delFFmpegSource?key=5f748d2ef9712e4b2f6f970c1d44d93a
func zlmConnectRtpServer(key string) (string, error) {
	params := map[string]string{}
	params["key"] = key

	var str string
	for k, v := range params {
		str += "&" + k + "=" + v
	}
	body, err := utils.GetRequest(m.MConfig.Media.RESTFUL + "/index/api/connectRtpServer?secret=" + m.MConfig.Media.Secret + str)
	if err != nil {
		logrus.Errorln("set server config failed, err=", err)
		return "", err
	}
	resp := map[string]interface{}{}
	err = utils.JSONDecode(body, &resp)
	if err != nil {
		logrus.Errorln("set config: json decode failed, err=", err)
		return "", err
	}
	if code, ok := resp["code"]; !ok || fmt.Sprint(code) != "0" {
		logrus.Errorln("set config: response error")
		return "", errors.New("set config: response error")
	}
	logrus.Infof("invoke interface zlm-connectRtpServer response:\n%s", string(body))
	return string(body), nil
}

// 功能：关闭ffmpeg拉流代理
//
// 范例：http://127.0.0.1/index/api/delFFmpegSource?key=5f748d2ef9712e4b2f6f970c1d44d93a
func zlmDelFFmpegSource(key string) (string, error) {
	params := map[string]string{}
	params["key"] = key
	var str string
	for k, v := range params {
		str += "&" + k + "=" + v
	}
	body, err := utils.GetRequest(m.MConfig.Media.RESTFUL + "/index/api/delFFmpegSource?secret=" + m.MConfig.Media.Secret + str)
	if err != nil {
		logrus.Errorln("set server config failed, err=", err)
		return "", err
	}
	resp := map[string]interface{}{}
	err = utils.JSONDecode(body, &resp)
	if err != nil {
		logrus.Errorln("set config: json decode failed, err=", err)
		return "", err
	}
	if code, ok := resp["code"]; !ok || fmt.Sprint(code) != "0" {
		logrus.Errorln("set config: response error")
		return "", errors.New("set config: response error")
	}
	logrus.Infof("invoke interface zlm-delFFmpegSource response:\n%s", string(body))
	return string(body), nil
}

// 功能：关闭拉流代理(流注册成功后，也可以使用close_streams接口替代)
//
// 范例：http://127.0.0.1/index/api/delStreamProxy?key=__defaultVhost__/proxy/0
func zlmDelStreamProxye(key string) (string, error) {
	params := map[string]string{}
	params["key"] = key
	var str string
	for k, v := range params {
		str += "&" + k + "=" + v
	}
	body, err := utils.GetRequest(m.MConfig.Media.RESTFUL + "/index/api/delStreamProxy?secret=" + m.MConfig.Media.Secret + str)
	if err != nil {
		logrus.Errorln("set server config failed, err=", err)
		return "", err
	}
	resp := map[string]interface{}{}
	err = utils.JSONDecode(body, &resp)
	if err != nil {
		logrus.Errorln("set config: json decode failed, err=", err)
		return "", err
	}
	if code, ok := resp["code"]; !ok || fmt.Sprint(code) != "0" {
		logrus.Errorln("set config: response error")
		return "", errors.New("set config: response error")
	}
	logrus.Infof("invoke interface zlm-delStreamProxy response:\n%s", string(body))
	return string(body), nil
}

// 功能：关闭推流
//
// 范例：http://127.0.0.1/index/api/delStreamPusherProxy?key=rtmp/defaultVhost/proxy/test/4AB43C9EABEB76AB443BB8260C8B2D12
func zlmDelStreamPusherProxy(key string) (string, error) {
	params := map[string]string{}
	params["key"] = key
	var str string
	for k, v := range params {
		str += "&" + k + "=" + v
	}
	body, err := utils.GetRequest(m.MConfig.Media.RESTFUL + "/index/api/delStreamPusherProxy?secret=" + m.MConfig.Media.Secret + str)
	if err != nil {
		logrus.Errorln("set server config failed, err=", err)
		return "", err
	}
	resp := map[string]interface{}{}
	err = utils.JSONDecode(body, &resp)
	if err != nil {
		logrus.Errorln("set config: json decode failed, err=", err)
		return "", err
	}
	if code, ok := resp["code"]; !ok || fmt.Sprint(code) != "0" {
		logrus.Errorln("set config: response error")
		return "", errors.New("set config: response error")
	}
	logrus.Infof("invoke interface zlm-delStreamPusherProxy response:\n%s", string(body))
	return string(body), nil
}

// 功能：获取所有TcpSession列表(获取所有tcp客户端相关信息)
//
// 范例：http://127.0.0.1/index/api/getAllSession
func zlmGetAllSession(local_port int, peer_id string) (string, error) {
	params := map[string]string{}
	params["local_port"] = string(local_port)
	params["peer_id"] = peer_id
	var str string
	for k, v := range params {
		str += "&" + k + "=" + v
	}
	body, err := utils.GetRequest(m.MConfig.Media.RESTFUL + "/index/api/getAllSession?secret=" + m.MConfig.Media.Secret + str)
	if err != nil {
		logrus.Errorln("set server config failed, err=", err)
		return "", err
	}
	resp := map[string]interface{}{}
	err = utils.JSONDecode(body, &resp)
	if err != nil {
		logrus.Errorln("set config: json decode failed, err=", err)
		return "", err
	}
	if code, ok := resp["code"]; !ok || fmt.Sprint(code) != "0" {
		logrus.Errorln("set config: response error")
		return "", errors.New("set config: response error")
	}
	logrus.Infof("invoke interface zlm-getAllSession response:\n%s", string(body))
	return string(body), nil
}

// 功能：获取API列表
//
// 范例：http://127.0.0.1/index/api/getApiList
func zlmGetApiList() (string, error) {
	body, err := utils.GetRequest(m.MConfig.Media.RESTFUL + "/index/api/getApiList?secret=" + m.MConfig.Media.Secret)
	if err != nil {
		logrus.Errorln("set server config failed, err=", err)
		return "", err
	}
	resp := map[string]interface{}{}
	err = utils.JSONDecode(body, &resp)
	if err != nil {
		logrus.Errorln("set config: json decode failed, err=", err)
		return "", err
	}
	if code, ok := resp["code"]; !ok || fmt.Sprint(code) != "0" {
		logrus.Errorln("set config: response error")
		return "", errors.New("set config: response error")
	}
	logrus.Infof("invoke interface zlm-getApiList response:\n%s", string(body))
	return string(body), nil
}

func zlmGetMediaPlayerList() (string, error) {
	body, err := utils.GetRequest(m.MConfig.Media.RESTFUL + "/index/api/getMediaPlayerList?secret=" + m.MConfig.Media.Secret)
	if err != nil {
		logrus.Errorln("set server config failed, err=", err)
		return "", err
	}
	resp := map[string]interface{}{}
	err = utils.JSONDecode(body, &resp)
	if err != nil {
		logrus.Errorln("set config: json decode failed, err=", err)
		return "", err
	}
	if code, ok := resp["code"]; !ok || fmt.Sprint(code) != "0" {
		logrus.Errorln("set config: response error")
		return "", errors.New("set config: response error")
	}
	logrus.Infof("invoke interface zlm-getMediaPlayerList response:\n%s", string(body))
	return string(body), nil
}

// 功能：搜索文件系统，获取流对应的录像文件列表或日期文件夹列表
//
// 范例：http://127.0.0.1/index/api/getMp4RecordFile?vhost=__defaultVhost__&app=live&stream=ss&period=2020-01
func zlmGetMp4RecordFile(vhost, app, stream, preriod, customized_path string) (string, error) {
	params := map[string]string{}
	params["vhost"] = vhost
	params["app"] = app
	params["stream"] = stream
	params["preriod"] = preriod
	params["customized_path"] = customized_path
	var str string
	for k, v := range params {
		str += "&" + k + "=" + v
	}
	body, err := utils.GetRequest(m.MConfig.Media.RESTFUL + "/index/api/getMp4RecordFile?secret=" + m.MConfig.Media.Secret)
	if err != nil {
		logrus.Errorln("set server config failed, err=", err)
		return "", err
	}
	resp := map[string]interface{}{}
	err = utils.JSONDecode(body, &resp)
	if err != nil {
		logrus.Errorln("set config: json decode failed, err=", err)
		return "", err
	}
	if code, ok := resp["code"]; !ok || fmt.Sprint(code) != "0" {
		logrus.Errorln("set config: response error")
		return "", errors.New("set config: response error")
	}
	logrus.Infof("invoke interface zlm-getMp4RecordFile response:\n%s", string(body))
	return string(body), nil
}

// 功能：获取服务器配置
//
// 范例：http://127.0.0.1/index/api/getServerConfig
func zlmGetServerConfig() (string, error) {
	body, err := utils.GetRequest(m.MConfig.Media.RESTFUL + "/index/api/getServerConfig?secret=" + m.MConfig.Media.Secret)
	if err != nil {
		logrus.Errorln("set server config failed, err=", err)
		return "", err
	}
	resp := map[string]interface{}{}
	err = utils.JSONDecode(body, &resp)
	if err != nil {
		logrus.Errorln("set config: json decode failed, err=", err)
		return "", err
	}
	if code, ok := resp["code"]; !ok || fmt.Sprint(code) != "0" {
		logrus.Errorln("set config: response error")
		return "", errors.New("set config: response error")
	}
	logrus.Infof("invoke interface zlm-getServerConfig response:\n%s", string(body))
	return string(body), nil
}

// 功能：获取截图或生成实时截图并返回
//
// 范例：http://127.0.0.1/index/api/getSnap?url=rtmp://127.0.0.1/record/robot.mp4&timeout_sec=10&expire_sec=30
func zlmGetSnap(url string, timeout_sec int, expire_sec int) (string, error) {
	params := map[string]string{}
	params["url"] = url
	params["timeout_sec"] = string(timeout_sec)
	params["expire_sec"] = string(expire_sec)

	var str string
	for k, v := range params {
		str += "&" + k + "=" + v
	}
	body, err := utils.GetRequest(m.MConfig.Media.RESTFUL + "/index/api/getSnap?secret=" + m.MConfig.Media.Secret)
	if err != nil {
		logrus.Errorln("set server config failed, err=", err)
		return "", err
	}
	resp := map[string]interface{}{}
	err = utils.JSONDecode(body, &resp)
	if err != nil {
		logrus.Errorln("set config: json decode failed, err=", err)
		return "", err
	}
	if code, ok := resp["code"]; !ok || fmt.Sprint(code) != "0" {
		logrus.Errorln("set config: response error")
		return "", errors.New("set config: response error")
	}
	logrus.Infof("invoke interface zlm-getSnap response:\n%s", string(body))
	return string(body), nil
}

// 功能：获取主要对象个数统计，主要用于分析内存性能
//
// 范例：http://127.0.0.1/index/api/getStatistic?secret=035c73f7-bb6b-4889-a715-d9eb2d1925cc
func zlmGetStatistic() (string, error) {
	body, err := utils.GetRequest(m.MConfig.Media.RESTFUL + "/index/api/getStatistic?secret=" + m.MConfig.Media.Secret)
	if err != nil {
		logrus.Errorln("set server config failed, err=", err)
		return "", err
	}
	resp := map[string]interface{}{}
	err = utils.JSONDecode(body, &resp)
	if err != nil {
		logrus.Errorln("set config: json decode failed, err=", err)
		return "", err
	}
	if code, ok := resp["code"]; !ok || fmt.Sprint(code) != "0" {
		logrus.Errorln("set config: response error")
		return "", errors.New("set config: response error")
	}
	logrus.Infof("invoke interface zlm-getStatistic response:\n%s", string(body))
	return string(body), nil
}

// 功能：获取各epoll(或select)线程负载以及延时
//
// 范例：http://127.0.0.1/index/api/getThreadsLoad
func zlmGetThreadsLoad() (string, error) {
	body, err := utils.GetRequest(m.MConfig.Media.RESTFUL + "/index/api/getThreadsLoad?secret=" + m.MConfig.Media.Secret)
	if err != nil {
		logrus.Errorln("set server config failed, err=", err)
		return "", err
	}
	resp := map[string]interface{}{}
	err = utils.JSONDecode(body, &resp)
	if err != nil {
		logrus.Errorln("set config: json decode failed, err=", err)
		return "", err
	}
	if code, ok := resp["code"]; !ok || fmt.Sprint(code) != "0" {
		logrus.Errorln("set config: response error")
		return "", errors.New("set config: response error")
	}
	logrus.Infof("invoke interface zlm-getThreadsLoad response:\n%s", string(body))
	return string(body), nil
}

// 功能：获取各后台epoll(或select)线程负载以及延时
//
// 范例：http://127.0.0.1/index/api/getWorkThreadsLoad
func zlmGetWorkThreadsLoad() (string, error) {
	body, err := utils.GetRequest(m.MConfig.Media.RESTFUL + "/index/api/getWorkThreadsLoad?secret=" + m.MConfig.Media.Secret)
	if err != nil {
		logrus.Errorln("set server config failed, err=", err)
		return "", err
	}
	resp := map[string]interface{}{}
	err = utils.JSONDecode(body, &resp)
	if err != nil {
		logrus.Errorln("set config: json decode failed, err=", err)
		return "", err
	}
	if code, ok := resp["code"]; !ok || fmt.Sprint(code) != "0" {
		logrus.Errorln("set config: response error")
		return "", errors.New("set config: response error")
	}
	logrus.Infof("invoke interface zlm-getWorkThreadsLoad response:\n%s", string(body))
	return string(body), nil
}

// 功能：判断直播流是否在线(已过期，请使用getMediaList接口替代)
//
// 范例：http://127.0.0.1/index/api/isMediaOnline?schema=rtsp&vhost=__defaultVhost__&app=live&stream=obs
func zlmIsMediaOnline(schema, vhost, app, stream string) (string, error) {
	params := map[string]string{}
	params["schema"] = schema
	params["vhost"] = vhost
	params["app"] = app
	params["stream"] = stream
	var str string
	for k, v := range params {
		str += "&" + k + "=" + v
	}
	body, err := utils.GetRequest(m.MConfig.Media.RESTFUL + "/index/api/isMediaOnline?secret=" + m.MConfig.Media.Secret + str)
	if err != nil {
		logrus.Errorln("set server config failed, err=", err)
		return "", err
	}
	resp := map[string]interface{}{}
	err = utils.JSONDecode(body, &resp)
	if err != nil {
		logrus.Errorln("set config: json decode failed, err=", err)
		return "", err
	}
	if code, ok := resp["code"]; !ok || fmt.Sprint(code) != "0" {
		logrus.Errorln("set config: response error")
		return "", errors.New("set config: response error")
	}
	logrus.Infof("invoke interface zlm-isMediaOnline response:\n%s", string(body))
	return string(body), nil
}

// 功能：获取流录制状态
//
// 范例：http://127.0.0.1/index/api/isRecording?type=1&vhost=__defaultVhost__&app=live&stream=obs
func zlmIsRecording(stype int, vhost, app, stream string) (string, error) {
	params := map[string]string{}
	params["type"] = string(stype)
	params["vhost"] = vhost
	params["app"] = app
	params["stream"] = stream
	var str string
	for k, v := range params {
		str += "&" + k + "=" + v
	}
	body, err := utils.GetRequest(m.MConfig.Media.RESTFUL + "/index/api/isRecording?secret=" + m.MConfig.Media.Secret + str)
	if err != nil {
		logrus.Errorln("set server config failed, err=", err)
		return "", err
	}
	resp := map[string]interface{}{}
	err = utils.JSONDecode(body, &resp)
	if err != nil {
		logrus.Errorln("set config: json decode failed, err=", err)
		return "", err
	}
	if code, ok := resp["code"]; !ok || fmt.Sprint(code) != "0" {
		logrus.Errorln("set config: response error")
		return "", errors.New("set config: response error")
	}
	logrus.Infof("invoke interface zlm-isRecording response:\n%s", string(body))
	return string(body), nil
}

// 功能：断开tcp连接，比如说可以断开rtsp、rtmp播放器等
//
// 范例：http://127.0.0.1/index/api/kick_session?id=140614440178720
func zlmKickSession(id string) (string, error) {
	params := map[string]string{}
	params["id"] = id

	var str string
	for k, v := range params {
		str += "&" + k + "=" + v
	}
	body, err := utils.GetRequest(m.MConfig.Media.RESTFUL + "/index/api/kick_session?secret=" + m.MConfig.Media.Secret + str)
	if err != nil {
		logrus.Errorln("set server config failed, err=", err)
		return "", err
	}
	resp := map[string]interface{}{}
	err = utils.JSONDecode(body, &resp)
	if err != nil {
		logrus.Errorln("set config: json decode failed, err=", err)
		return "", err
	}
	if code, ok := resp["code"]; !ok || fmt.Sprint(code) != "0" {
		logrus.Errorln("set config: response error")
		return "", errors.New("set config: response error")
	}
	logrus.Infof("invoke interface zlm-kick_session response:\n%s", string(body))
	return string(body), nil
}

// 功能：断开tcp连接，比如说可以断开rtsp、rtmp播放器等
//
// 范例：http://127.0.0.1/index/api/kick_sessions?local_port=554
func zlmKickSessions(local_port int, peer_ip string) (string, error) {
	params := map[string]string{}
	params["local_port"] = string(local_port)
	params["peer_ip"] = peer_ip

	var str string
	for k, v := range params {
		str += "&" + k + "=" + v
	}
	body, err := utils.GetRequest(m.MConfig.Media.RESTFUL + "/index/api/kick_sessions?secret=" + m.MConfig.Media.Secret + str)
	if err != nil {
		logrus.Errorln("set server config failed, err=", err)
		return "", err
	}
	resp := map[string]interface{}{}
	err = utils.JSONDecode(body, &resp)
	if err != nil {
		logrus.Errorln("set config: json decode failed, err=", err)
		return "", err
	}
	if code, ok := resp["code"]; !ok || fmt.Sprint(code) != "0" {
		logrus.Errorln("set config: response error")
		return "", errors.New("set config: response error")
	}
	logrus.Infof("invoke interface zlm-kick_sessions response:\n%s", string(body))
	return string(body), nil
}

// 功能：获取openRtpServer接口创建的所有RTP服务器
//
// 范例：http://127.0.0.1/index/api/listRtpServer
func zlmListRtpServer() (string, error) {
	body, err := utils.GetRequest(m.MConfig.Media.RESTFUL + "/index/api/listRtpServer?secret=" + m.MConfig.Media.Secret)
	if err != nil {
		logrus.Errorln("set server config failed, err=", err)
		return "", err
	}
	resp := map[string]interface{}{}
	err = utils.JSONDecode(body, &resp)
	if err != nil {
		logrus.Errorln("set config: json decode failed, err=", err)
		return "", err
	}
	if code, ok := resp["code"]; !ok || fmt.Sprint(code) != "0" {
		logrus.Errorln("set config: response error")
		return "", errors.New("set config: response error")
	}
	logrus.Infof("invoke interface zlm-listRtpServer response:\n%s", string(body))
	return string(body), nil
}

// 功能：创建GB28181 RTP接收端口，如果该端口接收数据超时，则会自动被回收(不用调用closeRtpServer接口)
//
// 范例：http://127.0.0.1/index/api/openRtpServer?port=0&tcp_mode=1&stream_id=test
func zlmOpenRtpServer(port, tcp_mode int, stream_id string) (string, error) {
	params := map[string]string{}
	params["port"] = string(port)
	params["tcp_mode"] = string(tcp_mode)
	params["stream_id"] = stream_id

	var str string
	for k, v := range params {
		str += "&" + k + "=" + v
	}
	body, err := utils.GetRequest(m.MConfig.Media.RESTFUL + "/index/api/openRtpServer?secret=" + m.MConfig.Media.Secret + str)
	if err != nil {
		logrus.Errorln("set server config failed, err=", err)
		return "", err
	}
	resp := map[string]interface{}{}
	err = utils.JSONDecode(body, &resp)
	if err != nil {
		logrus.Errorln("set config: json decode failed, err=", err)
		return "", err
	}
	if code, ok := resp["code"]; !ok || fmt.Sprint(code) != "0" {
		logrus.Errorln("set config: response error")
		return "", errors.New("set config: response error")
	}
	logrus.Infof("invoke interface zlm-openRtpServer response:\n%s", string(body))
	return string(body), nil
}

// 功能：重启服务器,只有Daemon方式才能重启，否则是直接关闭！
//
// 范例：http://127.0.0.1/index/api/restartServer
func zlmRestartServer() (string, error) {
	body, err := utils.GetRequest(config.Media.RESTFUL + "/index/api/restartServer?secret=" + config.Media.Secret)
	if err != nil {
		logrus.Errorln("set server config failed, err=", err)
		return "", err
	}
	resp := map[string]interface{}{}
	err = utils.JSONDecode(body, &resp)
	if err != nil {
		logrus.Errorln("set config: json decode failed, err=", err)
		return "", err
	}
	if code, ok := resp["code"]; !ok || fmt.Sprint(code) != "0" {
		logrus.Errorln("set config: response error")
		return "", errors.New("set config: response error")
	}
	logrus.Infof("invoke interface zlm-restartServer response:\n%s", string(body))
	return string(body), nil
}

// 功能：作为GB28181客户端，启动ps-rtp推流，支持rtp/udp方式；该接口支持rtsp/rtmp等协议转ps-rtp推流。第一次推流失败会直接返回错误，成功一次后，后续失败也将无限重试。
//
// 范例：http://127.0.0.1/index/api/startSendRtp?secret=035c73f7-bb6b-4889-a715-d9eb2d1925cc&vhost=__defaultVhost__&app=live&stream=test&ssrc=1&dst_url=127.0.0.1&dst_port=10000&is_udp=0
func zlmStartSendRtp(vhost, app, stream, ssrc, dst_url string, dst_port int, is_udp bool, src_port int, pt int8, use_ps int, only_audio int) (string, error) {
	params := map[string]string{}
	params["vhost"] = vhost
	params["app"] = app
	params["stream"] = stream
	params["ssrc"] = ssrc
	params["dst_url"] = dst_url
	params["dst_port"] = string(dst_port)
	params["is_udp"] = strconv.FormatBool(is_udp)
	params["src_port"] = string(src_port)
	params["pt"] = string(pt)
	params["use_ps"] = string(use_ps)
	params["only_audio"] = string(only_audio)

	var str string
	for k, v := range params {
		str += "&" + k + "=" + v
	}
	body, err := utils.GetRequest(m.MConfig.Media.RESTFUL + "/index/api/startSendRtp?secret=" + m.MConfig.Media.Secret)
	if err != nil {
		logrus.Errorln("set server config failed, err=", err)
		return "", err
	}
	resp := map[string]interface{}{}
	err = utils.JSONDecode(body, &resp)
	if err != nil {
		logrus.Errorln("set config: json decode failed, err=", err)
		return "", err
	}
	if code, ok := resp["code"]; !ok || fmt.Sprint(code) != "0" {
		logrus.Errorln("set config: response error")
		return "", errors.New("set config: response error")
	}
	logrus.Infof("invoke interface zlm-startSendRtp response:\n%s", string(body))
	return string(body), nil
}

// 功能：作为GB28181 Passive TCP服务器；该接口支持rtsp/rtmp等协议转ps-rtp被动推流。调用该接口，zlm会启动tcp服务器等待连接请求，连接建立后，zlm会关闭tcp服务器，然后源源不断的往客户端推流。第一次推流失败会直接返回错误，成功一次后，后续失败也将无限重试(不停地建立tcp监听，超时后再关闭)。
//
// 范例：http://127.0.0.1/index/api/startSendRtpPassive?secret=035c73f7-bb6b-4889-a715-d9eb2d1925cc&vhost=__defaultVhost__&app=live&stream=test&ssrc=1
func zlmStartSendRtpPassive(vhost, app, stream, ssrc, dst_url string, dst_port int, is_udp bool, src_port int, pt int8, use_ps int, only_audio int) (string, error) {
	params := map[string]string{}
	params["vhost"] = vhost
	params["app"] = app
	params["stream"] = stream
	params["ssrc"] = ssrc

	params["dst_url"] = dst_url
	params["dst_port"] = string(dst_port)
	params["is_udp"] = strconv.FormatBool(is_udp)
	params["src_port"] = string(src_port)

	params["pt"] = string(pt)
	params["use_ps"] = string(use_ps)
	params["only_audio"] = string(only_audio)

	var str string
	for k, v := range params {
		str += "&" + k + "=" + v
	}
	body, err := utils.GetRequest(m.MConfig.Media.RESTFUL + "/index/api/startSendRtpPassive?secret=" + m.MConfig.Media.Secret + str)
	if err != nil {
		logrus.Errorln("set server config failed, err=", err)
		return "", err
	}
	resp := map[string]interface{}{}
	err = utils.JSONDecode(body, &resp)
	if err != nil {
		logrus.Errorln("set config: json decode failed, err=", err)
		return "", err
	}
	if code, ok := resp["code"]; !ok || fmt.Sprint(code) != "0" {
		logrus.Errorln("set config: response error")
		return "", errors.New("set config: response error")
	}
	logrus.Infof("invoke interface zlm-startSendRtpPassive response:\n%s", string(body))
	return string(body), nil
}

// 功能：停止GB28181 ps-rtp推流
//
// 范例：http://127.0.0.1/index/api/stopSendRtp?secret=035c73f7-bb6b-4889-a715-d9eb2d1925cc&vhost=__defaultVhost__&app=live&stream=test
func zlmStopSendRtp(vhost, app, stream, ssrc string) (string, error) {
	params := map[string]string{}
	params["vhost"] = vhost
	params["app"] = app
	params["stream"] = stream
	params["ssrc"] = ssrc
	var str string
	for k, v := range params {
		str += "&" + k + "=" + v
	}
	body, err := utils.GetRequest(m.MConfig.Media.RESTFUL + "/index/api/stopSendRtp?secret=" + m.MConfig.Media.Secret + str)
	if err != nil {
		logrus.Errorln("set server config failed, err=", err)
		return "", err
	}
	resp := map[string]interface{}{}
	err = utils.JSONDecode(body, &resp)
	if err != nil {
		logrus.Errorln("set config: json decode failed, err=", err)
		return "", err
	}
	if code, ok := resp["code"]; !ok || fmt.Sprint(code) != "0" {
		logrus.Errorln("set config: response error")
		return "", errors.New("set config: response error")
	}
	logrus.Infof("invoke interface zlm-stopSendRtp response:\n%s", string(body))
	return string(body), nil
}

// 功能：获取版本信息，如分支，commit id, 编译时间
//
// 范例：http://127.0.0.1/index/api/version
func ZlmVersion() (map[string]interface{}, error) {
	body, err := utils.GetRequest(m.MConfig.Media.RESTFUL + "/index/api/version?secret=" + m.MConfig.Media.Secret)
	if err != nil {
		logrus.Errorln("set server config failed, err=", err)
		return nil, err
	}
	resp := map[string]interface{}{}
	err = utils.JSONDecode(body, &resp)
	if err != nil {
		logrus.Errorln("set config: json decode failed, err=", err)
		return nil, err
	}
	if code, ok := resp["code"]; !ok || fmt.Sprint(code) != "0" {
		logrus.Errorln("set config: response error")
		return nil, errors.New("set config: response error")
	}
	logrus.Infof("invoke interface zlm-version response:\n%s", string(body))
	return resp, nil
}

// 功能：设置zlm配置,单个媒体服务器
//
// 范例：http://127.0.0.1/index/api/setServerConfig?api.apiDebug=0(例如关闭http api调试)
func ZlmSetServerConfig4Single() error {
	hookURL := fmt.Sprintf("http://%s/index/hook/", m.MConfig.API)
	params := map[string]string{}
	params["hook.enable"] = "1"
	params["hook.on_flow_report"] = hookURL + "on_flow_report"
	params["hook.on_http_access"] = hookURL + "on_http_access"
	params["hook.on_play"] = hookURL + "on_play"
	params["hook.on_publish"] = hookURL + "on_publish"
	params["hook.on_record_mp4"] = hookURL + "on_record_mp4"
	params["hook.on_record_ts"] = hookURL + "on_record_ts"
	params["hook.on_rtp_server_timeout"] = hookURL + "on_rtp_server_timeout"
	params["hook.on_rtsp_auth"] = hookURL + "on_rtsp_auth"
	params["hook.on_rtsp_realm"] = hookURL + "on_rtsp_realm"
	params["hook.on_send_rtp_stopped"] = hookURL + "on_send_rtp_stopped"
	params["hook.on_server_keepalive"] = hookURL + "on_server_keepalive"
	params["hook.on_server_started"] = hookURL + "on_server_started"
	params["hook.on_shell_login"] = hookURL + "on_shell_login"
	params["hook.on_stream_changed"] = hookURL + "on_stream_changed"
	params["hook.on_stream_none_reader"] = hookURL + "on_stream_none_reader"
	params["hook.on_stream_not_found"] = hookURL + "on_stream_not_found"
	var str string
	for k, v := range params {
		str += "&" + k + "=" + v
	}
	body, err := utils.GetRequest(m.MConfig.Media.RESTFUL + "/index/api/setServerConfig?secret=" + m.MConfig.Media.Secret + str)
	if err != nil {
		logrus.Errorln("set server config failed, err=", err)
		return err
	}
	resp := map[string]interface{}{}
	err = utils.JSONDecode(body, &resp)
	if err != nil {
		logrus.Errorln("set config: json decode failed, err=", err)
		return err
	}
	if code, ok := resp["code"]; !ok || fmt.Sprint(code) != "0" {
		logrus.Errorln("set config: response error")
		return errors.New("set config: response error")
	}
	logrus.Infof("invoke interface zlm-setServerConfig response:\n%s", string(body))
	return nil
}

/*
// 功能：设置zlm配置,多个媒体服务器
//
// 范例：http://127.0.0.1/index/api/setServerConfig?api.apiDebug=0(例如关闭http api调试)
func zlmSetServerConfig4Multi(servers []*ServerItem) error {

	hookURL := fmt.Sprintf("http://%s/index/hook/", config.API)
	params := map[string]string{}
	params["hook.enable"] = "1"
	params["hook.on_flow_report"] = hookURL + "on_flow_report"
	params["hook.on_play"] = hookURL + "on_play"
	params["hook.on_publish"] = hookURL + "on_publish"
	params["hook.on_server_started"] = hookURL + "on_server_started"
	params["hook.on_shell_login"] = hookURL + "on_shell_login"
	params["hook.on_stream_changed"] = hookURL + "on_stream_changed"
	params["hook.on_stream_none_reader"] = hookURL + "on_stream_none_reader"
	params["hook.on_stream_not_found"] = hookURL + "on_stream_not_found"
	var str string
	for k, v := range params {
		str += "&" + k + "=" + v
	}

	for _, s := range servers {

		body, err := utils.GetRequest(config.Media.RESTFUL + "/index/api/setServerConfig?secret=" + config.Media.Secret + str)
		if err != nil {
			logrus.Errorln("set server config failed, err=", err)
			continue
		}
		resp := map[string]interface{}{}
		err = utils.JSONDecode(body, &resp)
		if err != nil {
			logrus.Errorln("set config: json decode failed, err=", err)
			continue
		}
		if code, ok := resp["code"]; !ok || fmt.Sprint(code) != "0" {
			logrus.Errorln("set config: response error")
			continue
		}
	}
	return nil
}*/
