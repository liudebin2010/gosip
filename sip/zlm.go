package sipapi

import (
	"errors"
	"fmt"
	"github.com/panjjo/gosip/utils"
	"github.com/sirupsen/logrus"
	"net/url"
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
func ZlmAddFFmpegSource(params map[string]string) (map[string]interface{}, error) {
	var str string
	for k, v := range params {
		str += "&" + k + "=" + v
	}
	body, err := utils.GetRequest(config.Media.RESTFUL + "/index/api/addFFmpegSource?secret=" + config.Media.Secret + str)
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
	logrus.Infof("invoke interface zlm-addFFmpegSource response:\n%s", resp)
	return resp, nil
}

// 功能：动态添加rtsp/rtmp/hls/http-ts/http-flv拉流代理(只支持H264/H265/aac/G711/opus负载)
// 范例：http://127.0.0.1/index/api/addStreamProxy?vhost=__defaultVhost__&app=proxy&stream=0&url=rtmp://live.hkstv.hk.lxdns.com/live/hks2
func ZlmAddStreamProxy(params map[string]string) (map[string]interface{}, error) {
	var str string
	for k, v := range params {
		str += "&" + k + "=" + v
	}
	body, err := utils.GetRequest(config.Media.RESTFUL + "/index/api/addStreamProxy?secret=" + config.Media.Secret)
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
	logrus.Infof("invoke interface zlm-addStreamProxy response:\n%s", resp)
	return resp, nil
}

// 功能：添加rtsp/rtmp主动推流(把本服务器的直播流推送到其他服务器去)
//
// 范例：http://127.0.0.1/index/api/addStreamPusherProxy?vhost=__defaultVhost__&app=proxy&stream=test&dst_url=rtmp://127.0.0.1/live/test2
func ZlmAddStreamPusherProxy(params map[string]string) (map[string]interface{}, error) {
	var str string
	for k, v := range params {
		str += "&" + k + "=" + v
	}
	body, err := utils.GetRequest(config.Media.RESTFUL + "/index/api/addStreamPusherProxy?secret=" + config.Media.Secret + str)
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
	logrus.Infof("invoke interface zlm-addStreamPusherProxy response:\n%s", resp)
	return resp, nil
}

// 功能：创建GB28181 RTP接收端口，如果该端口接收数据超时，则会自动被回收(不用调用closeRtpServer接口)
//
// 范例：http://127.0.0.1/index/api/openRtpServer?port=0&tcp_mode=1&stream_id=test
func ZlmCloseRtpServer(params map[string]string) (map[string]interface{}, error) {
	var str string
	for k, v := range params {
		str += "&" + k + "=" + v
	}
	body, err := utils.GetRequest(config.Media.RESTFUL + "/index/api/closeRtpServer?secret=" + config.Media.Secret + str)
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
	logrus.Infof("invoke interface zlm-closeRtpServer response:\n%s", resp)
	return resp, nil
}

// 功能：关闭流(目前所有类型的流都支持关闭)
//
// 范例：http://127.0.0.1/index/api/close_streams?schema=rtmp&vhost=__defaultVhost__&app=live&stream=0&force=1
func ZlmCloseStreams(params map[string]string) (map[string]interface{}, error) {
	var str string
	for k, v := range params {
		str += "&" + k + "=" + v
	}
	body, err := utils.GetRequest(config.Media.RESTFUL + "/index/api/close_streams?secret=" + config.Media.Secret + str)
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
	logrus.Infof("invoke interface zlm-close_streams response:\n%s", resp)
	return resp, nil
}

// 功能：关闭ffmpeg拉流代理
//
// 范例：http://127.0.0.1/index/api/delFFmpegSource?key=5f748d2ef9712e4b2f6f970c1d44d93a
func ZlmConnectRtpServer(params map[string]string) (map[string]interface{}, error) {

	var str string
	for k, v := range params {
		str += "&" + k + "=" + v
	}
	body, err := utils.GetRequest(config.Media.RESTFUL + "/index/api/connectRtpServer?secret=" + config.Media.Secret + str)
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
	logrus.Infof("invoke interface zlm-connectRtpServer response:\n%s", resp)
	return resp, nil
}

// 功能：关闭ffmpeg拉流代理
//
// 范例：http://127.0.0.1/index/api/delFFmpegSource?key=5f748d2ef9712e4b2f6f970c1d44d93a
func ZlmDelFFmpegSource(params map[string]string) (map[string]interface{}, error) {
	var str string
	for k, v := range params {
		str += "&" + k + "=" + v
	}
	body, err := utils.GetRequest(config.Media.RESTFUL + "/index/api/delFFmpegSource?secret=" + config.Media.Secret + str)
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
	logrus.Infof("invoke interface zlm-delFFmpegSource response:\n%s", resp)
	return resp, nil
}

// 功能：关闭拉流代理(流注册成功后，也可以使用close_streams接口替代)
//
// 范例：http://127.0.0.1/index/api/delStreamProxy?key=__defaultVhost__/proxy/0
func ZlmDelStreamProxye(params map[string]string) (map[string]interface{}, error) {
	var str string
	for k, v := range params {
		str += "&" + k + "=" + v
	}
	body, err := utils.GetRequest(config.Media.RESTFUL + "/index/api/delStreamProxy?secret=" + config.Media.Secret + str)
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
	logrus.Infof("invoke interface zlm-delStreamProxy response:\n%s", resp)
	return resp, nil
}

// 功能：关闭推流
//
// 范例：http://127.0.0.1/index/api/delStreamPusherProxy?key=rtmp/defaultVhost/proxy/test/4AB43C9EABEB76AB443BB8260C8B2D12
func ZlmDelStreamPusherProxy(params map[string]string) (map[string]interface{}, error) {
	var str string
	for k, v := range params {
		str += "&" + k + "=" + v
	}
	body, err := utils.GetRequest(config.Media.RESTFUL + "/index/api/delStreamPusherProxy?secret=" + config.Media.Secret + str)
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
	logrus.Infof("invoke interface zlm-delStreamPusherProxy response:\n%s", resp)
	return resp, nil
}

// 功能：获取所有TcpSession列表(获取所有tcp客户端相关信息)
//
// 范例：http://127.0.0.1/index/api/getAllSession
func ZlmGetAllSession(params map[string]string) (map[string]interface{}, error) {
	var str string
	for k, v := range params {
		str += "&" + k + "=" + v
	}
	body, err := utils.GetRequest(config.Media.RESTFUL + "/index/api/getAllSession?secret=" + config.Media.Secret + str)
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
	logrus.Infof("invoke interface zlm-getAllSession response:\n%s", resp)
	return resp, nil
}

// 功能：获取API列表
//
// 范例：http://127.0.0.1/index/api/getApiList
func ZlmGetApiList() (map[string]interface{}, error) {
	body, err := utils.GetRequest(config.Media.RESTFUL + "/index/api/getApiList?secret=" + config.Media.Secret)
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
	logrus.Infof("invoke interface zlm-getApiList response:\n%s", resp)
	return resp, nil
}

func ZlmGetMediaPlayerList() (map[string]interface{}, error) {
	body, err := utils.GetRequest(config.Media.RESTFUL + "/index/api/getMediaPlayerList?secret=" + config.Media.Secret)
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
	logrus.Infof("invoke interface zlm-getMediaPlayerList response:\n%s", resp)
	return resp, nil
}

// 功能：搜索文件系统，获取流对应的录像文件列表或日期文件夹列表
//
// 范例：http://127.0.0.1/index/api/getMp4RecordFile?vhost=__defaultVhost__&app=live&stream=ss&period=2020-01
func ZlmGetMp4RecordFile(params map[string]string) (map[string]interface{}, error) {
	var str string
	for k, v := range params {
		str += "&" + k + "=" + v
	}
	body, err := utils.GetRequest(config.Media.RESTFUL + "/index/api/getMp4RecordFile?secret=" + config.Media.Secret)
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
	logrus.Infof("invoke interface zlm-getMp4RecordFile response:\n%s", resp)
	return resp, nil
}

// 功能：获取服务器配置
//
// 范例：http://127.0.0.1/index/api/getServerConfig
func ZlmGetServerConfig() (map[string]interface{}, error) {
	body, err := utils.GetRequest(config.Media.RESTFUL + "/index/api/getServerConfig?secret=" + config.Media.Secret)
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
	logrus.Infof("invoke interface zlm-getServerConfig response:\n%s", resp)
	return resp, nil
}

// 功能：获取截图或生成实时截图并返回
//
// 范例：http://127.0.0.1/index/api/getSnap?url=rtmp://127.0.0.1/record/robot.mp4&timeout_sec=10&expire_sec=30
func ZlmGetSnap(params map[string]string) (map[string]interface{}, error) {
	var str string
	for k, v := range params {
		str += "&" + k + "=" + v
	}
	body, err := utils.GetRequest(config.Media.RESTFUL + "/index/api/getSnap?secret=" + config.Media.Secret)
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
	logrus.Infof("invoke interface zlm-getSnap response:\n%s", resp)
	return resp, nil
}

// 功能：获取主要对象个数统计，主要用于分析内存性能
//
// 范例：http://127.0.0.1/index/api/getStatistic?secret=035c73f7-bb6b-4889-a715-d9eb2d1925cc
func ZlmGetStatistic() (map[string]interface{}, error) {
	body, err := utils.GetRequest(config.Media.RESTFUL + "/index/api/getStatistic?secret=" + config.Media.Secret)
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
	logrus.Infof("invoke interface zlm-getStatistic response:\n%s", resp)
	return resp, nil
}

// 功能：获取各epoll(或select)线程负载以及延时
//
// 范例：http://127.0.0.1/index/api/getThreadsLoad
func ZlmGetThreadsLoad() (map[string]interface{}, error) {
	body, err := utils.GetRequest(config.Media.RESTFUL + "/index/api/getThreadsLoad?secret=" + config.Media.Secret)
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
	logrus.Infof("invoke interface zlm-getThreadsLoad response:\n%s", resp)
	return resp, nil
}

// 功能：获取各后台epoll(或select)线程负载以及延时
//
// 范例：http://127.0.0.1/index/api/getWorkThreadsLoad
func ZlmGetWorkThreadsLoad() (map[string]interface{}, error) {
	body, err := utils.GetRequest(config.Media.RESTFUL + "/index/api/getWorkThreadsLoad?secret=" + config.Media.Secret)
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
	logrus.Infof("invoke interface zlm-getWorkThreadsLoad response:\n%s", resp)
	return resp, nil
}

// 功能：判断直播流是否在线(已过期，请使用getMediaList接口替代)
//
// 范例：http://127.0.0.1/index/api/isMediaOnline?schema=rtsp&vhost=__defaultVhost__&app=live&stream=obs
func ZlmIsMediaOnline(params map[string]string) (map[string]interface{}, error) {
	var str string
	for k, v := range params {
		str += "&" + k + "=" + v
	}
	body, err := utils.GetRequest(config.Media.RESTFUL + "/index/api/isMediaOnline?secret=" + config.Media.Secret + str)
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
	logrus.Infof("invoke interface zlm-isMediaOnline response:\n%s", resp)
	return resp, nil
}

// 功能：获取流录制状态
//
// 范例：http://127.0.0.1/index/api/isRecording?type=1&vhost=__defaultVhost__&app=live&stream=obs
func ZlmIsRecording(params map[string]string) (map[string]interface{}, error) {
	var str string
	for k, v := range params {
		str += "&" + k + "=" + v
	}
	body, err := utils.GetRequest(config.Media.RESTFUL + "/index/api/isRecording?secret=" + config.Media.Secret + str)
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
	logrus.Infof("invoke interface zlm-isRecording response:\n%s", resp)
	return resp, nil
}

// 功能：断开tcp连接，比如说可以断开rtsp、rtmp播放器等
//
// 范例：http://127.0.0.1/index/api/kick_session?id=140614440178720
func ZlmKickSession(params map[string]string) (map[string]interface{}, error) {
	var str string
	for k, v := range params {
		str += "&" + k + "=" + v
	}
	body, err := utils.GetRequest(config.Media.RESTFUL + "/index/api/kick_session?secret=" + config.Media.Secret + str)
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
	logrus.Infof("invoke interface zlm-kick_session response:\n%s", resp)
	return resp, nil
}

// 功能：断开tcp连接，比如说可以断开rtsp、rtmp播放器等
//
// 范例：http://127.0.0.1/index/api/kick_sessions?local_port=554
func ZlmKickSessions(params map[string]string) (map[string]interface{}, error) {
	var str string
	for k, v := range params {
		str += "&" + k + "=" + v
	}
	body, err := utils.GetRequest(config.Media.RESTFUL + "/index/api/kick_sessions?secret=" + config.Media.Secret + str)
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
	logrus.Infof("invoke interface zlm-kick_sessions response:\n%s", resp)
	return resp, nil
}

// 功能：获取openRtpServer接口创建的所有RTP服务器
//
// 范例：http://127.0.0.1/index/api/listRtpServer
func ZlmListRtpServer() (map[string]interface{}, error) {
	body, err := utils.GetRequest(config.Media.RESTFUL + "/index/api/listRtpServer?secret=" + config.Media.Secret)
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
	logrus.Infof("invoke interface zlm-listRtpServer response:\n%s", resp)
	return resp, nil
}

// 功能：创建GB28181 RTP接收端口，如果该端口接收数据超时，则会自动被回收(不用调用closeRtpServer接口)
//
// 范例：http://127.0.0.1/index/api/openRtpServer?port=0&tcp_mode=1&stream_id=test
func ZlmOpenRtpServer(params map[string]string) (map[string]interface{}, error) {
	var str string
	for k, v := range params {
		str += "&" + k + "=" + v
	}
	body, err := utils.GetRequest(config.Media.RESTFUL + "/index/api/openRtpServer?secret=" + config.Media.Secret + str)
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
	logrus.Infof("invoke interface zlm-openRtpServer response:\n%s", resp)
	return resp, nil
}

// 功能：重启服务器,只有Daemon方式才能重启，否则是直接关闭！
//
// 范例：http://127.0.0.1/index/api/restartServer
func ZlmRestartServer() (map[string]interface{}, error) {
	body, err := utils.GetRequest(config.Media.RESTFUL + "/index/api/restartServer?secret=" + config.Media.Secret)
	if err != nil {
		logrus.Errorln("ZlmRestartServer failed, err=", err)
		return nil, err
	}
	resp := map[string]interface{}{}
	err = utils.JSONDecode(body, &resp)
	if err != nil {
		logrus.Errorln("ZlmRestartServer: json decode failed, err=", err)
		return nil, err
	}
	if code, ok := resp["code"]; !ok || fmt.Sprint(code) != "0" {
		logrus.Errorln("ZlmRestartServer: response error")
		return nil, errors.New("ZlmRestartServer: response error")
	}
	logrus.Infof("ZlmRestartServer response:\n%s", resp)
	return resp, nil
}

// 功能：作为GB28181客户端，启动ps-rtp推流，支持rtp/udp方式；该接口支持rtsp/rtmp等协议转ps-rtp推流。第一次推流失败会直接返回错误，成功一次后，后续失败也将无限重试。
//
// 范例：http://127.0.0.1/index/api/startSendRtp?secret=035c73f7-bb6b-4889-a715-d9eb2d1925cc&vhost=__defaultVhost__&app=live&stream=test&ssrc=1&dst_url=127.0.0.1&dst_port=10000&is_udp=0
func ZlmStartSendRtp(params map[string]string) (map[string]interface{}, error) {
	var str string
	for k, v := range params {
		str += "&" + k + "=" + v
	}
	body, err := utils.GetRequest(config.Media.RESTFUL + "/index/api/startSendRtp?secret=" + config.Media.Secret)
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
	logrus.Infof("invoke interface zlm-startSendRtp response:\n%s", resp)
	return resp, nil
}

// 功能：作为GB28181 Passive TCP服务器；该接口支持rtsp/rtmp等协议转ps-rtp被动推流。调用该接口，zlm会启动tcp服务器等待连接请求，连接建立后，zlm会关闭tcp服务器，然后源源不断的往客户端推流。第一次推流失败会直接返回错误，成功一次后，后续失败也将无限重试(不停地建立tcp监听，超时后再关闭)。
//
// 范例：http://127.0.0.1/index/api/startSendRtpPassive?secret=035c73f7-bb6b-4889-a715-d9eb2d1925cc&vhost=__defaultVhost__&app=live&stream=test&ssrc=1
func ZlmStartSendRtpPassive(params map[string]string) (map[string]interface{}, error) {
	var str string
	for k, v := range params {
		str += "&" + k + "=" + v
	}
	body, err := utils.GetRequest(config.Media.RESTFUL + "/index/api/startSendRtpPassive?secret=" + config.Media.Secret + str)
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
	logrus.Infof("invoke interface zlm-startSendRtpPassive response:\n%s", resp)
	return resp, nil
}

// 功能：停止GB28181 ps-rtp推流
//
// 范例：http://127.0.0.1/index/api/stopSendRtp?secret=035c73f7-bb6b-4889-a715-d9eb2d1925cc&vhost=__defaultVhost__&app=live&stream=test
func ZlmStopSendRtp(params map[string]string) (map[string]interface{}, error) {
	var str string
	for k, v := range params {
		str += "&" + k + "=" + v
	}
	body, err := utils.GetRequest(config.Media.RESTFUL + "/index/api/stopSendRtp?secret=" + config.Media.Secret + str)
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
	logrus.Infof("invoke interface zlm-stopSendRtp response:\n%s", resp)
	return resp, nil
}

// 功能：获取版本信息，如分支，commit id, 编译时间
//
// 范例：http://127.0.0.1/index/api/version
func ZlmVersion() (map[string]interface{}, error) {
	body, err := utils.GetRequest(config.Media.RESTFUL + "/index/api/version?secret=" + config.Media.Secret)
	if err != nil {
		logrus.Errorln("ZlmVersion failed, err=", err)
		return nil, err
	}
	resp := map[string]interface{}{}
	err = utils.JSONDecode(body, &resp)
	if err != nil {
		logrus.Errorln("ZlmVersion: json decode failed, err=", err)
		return nil, err
	}
	if code, ok := resp["code"]; !ok || fmt.Sprint(code) != "0" {
		logrus.Errorln("ZlmVersion: response error")
		return nil, errors.New("ZlmVersion: response error")
	}
	logrus.Infof("ZlmVersion response:\n%s", resp)
	return resp, nil
}

// 功能：设置zlm配置
//
// 范例：http://127.0.0.1/index/api/setServerConfig?api.apiDebug=0(例如关闭http api调试)
func ZlmSetServerConfig(params map[string]string) (map[string]interface{}, error) {
	var str string
	for k, v := range params {
		str += "&" + k + "=" + v
	}
	body, err := utils.GetRequest(config.Media.RESTFUL + "/index/api/setServerConfig?secret=" + config.Media.Secret + str)
	if err != nil {
		logrus.Errorln("ZlmSetServerConfig failed, err=", err)
		return nil, err
	}
	resp := map[string]interface{}{}
	err = utils.JSONDecode(body, &resp)
	if err != nil {
		logrus.Errorln("ZlmSetServerConfig: json decode failed, err=", err)
		return nil, err
	}
	if code, ok := resp["code"]; !ok || fmt.Sprint(code) != "0" {
		logrus.Errorln("ZlmSetServerConfig: response error")
		return nil, errors.New("ZlmSetServerConfig: response error")
	}
	logrus.Infof("ZlmSetServerConfig response:\n%s", resp)
	return resp, nil
}

// 功能: 同步hook到zlm配置文件
func SyncWebhook2ZlmConfig() (map[string]interface{}, error) {
	hookURL := fmt.Sprintf("http://%s/index/hook/", config.API)
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
	body, err := utils.GetRequest(config.Media.RESTFUL + "/index/api/setServerConfig?secret=" + config.Media.Secret + str)
	if err != nil {
		logrus.Errorln("SyncWebhook2ZlmConfig failed, err=", err)
		return nil, err
	}
	resp := map[string]interface{}{}
	err = utils.JSONDecode(body, &resp)
	if err != nil {
		logrus.Errorln("SyncWebhook2ZlmConfig: json decode failed, err=", err)
		return nil, err
	}
	if code, ok := resp["code"]; !ok || fmt.Sprint(code) != "0" {
		logrus.Errorln("SyncWebhook2ZlmConfig: response error")
		return nil, errors.New("SyncWebhook2ZlmConfig: response error")
	}
	logrus.Infof("SyncWebhook2ZlmConfig response:\n%s", resp)
	return resp, nil
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
