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
	tmp := map[string]any{}
	err = utils.JSONDecode(body, &tmp)
	if err != nil {
		return err
	}
	if _, ok := tmp["code"]; !ok {
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
	tmp := map[string]any{}
	err = utils.JSONDecode(body, &tmp)
	if err != nil {
		return err
	}
	if _, ok := tmp["code"]; !ok {
		return utils.NewError(nil, tmp)
	}
	return nil
}

// 功能：通过fork FFmpeg进程的方式拉流代理，支持任意协议
//
// 范例：http://127.0.0.1/index/api/addFFmpegSource?src_url=http://live.hkstv.hk.lxdns.com/live/hks2/playlist.m3u8&dst_url=rtmp://127.0.0.1/live/hks2&timeout_ms=10000&ffmpeg_cmd_key=ffmpeg.cmd
func ZlmAddFFmpegSource(params map[string]any) (map[string]any, error) {
	body, err := utils.PostJSONRequest(config.Media.RESTFUL+"/index/api/addFFmpegSource", params)
	if err != nil {
		logrus.Errorln("ZlmAddFFmpegSource failed, err=", err)
		return nil, err
	}
	resp := map[string]any{}
	err = utils.JSONDecode(body, &resp)
	if err != nil {
		logrus.Errorln("ZlmAddFFmpegSource: json decode failed, err=", err)
		return nil, err
	}
	if _, ok := resp["code"]; !ok {
		logrus.Errorln("ZlmAddFFmpegSource: response error")
		return nil, errors.New("ZlmAddFFmpegSource: response error")
	}
	logrus.Infof("invoke api ZlmAddFFmpegSource response:\n%s", resp)
	return resp, nil
}

// 功能：动态添加rtsp/rtmp/hls/http-ts/http-flv拉流代理(只支持H264/H265/aac/G711/opus负载)
// 范例：http://127.0.0.1/index/api/addStreamProxy?vhost=__defaultVhost__&app=proxy&stream=0&url=rtmp://live.hkstv.hk.lxdns.com/live/hks2
func ZlmAddStreamProxy(params map[string]any) (map[string]any, error) {
	body, err := utils.PostJSONRequest(config.Media.RESTFUL+"/index/api/addStreamProxy", params)
	if err != nil {
		logrus.Errorln("ZlmAddStreamProxy, err=", err)
		return nil, err
	}
	resp := map[string]any{}
	err = utils.JSONDecode(body, &resp)
	if err != nil {
		logrus.Errorln("ZlmAddStreamProxy: json decode failed, err=", err)
		return nil, err
	}
	if _, ok := resp["code"]; !ok {
		logrus.Errorln("ZlmAddStreamProxy: response error")
		return nil, errors.New("set config: response error")
	}
	logrus.Infof("invoke api ZlmAddStreamProxy response:\n%s", resp)
	return resp, nil
}

// 功能：添加rtsp/rtmp主动推流(把本服务器的直播流推送到其他服务器去)
//
// 范例：http://127.0.0.1/index/api/addStreamPusherProxy?vhost=__defaultVhost__&app=proxy&stream=test&dst_url=rtmp://127.0.0.1/live/test2
func ZlmAddStreamPusherProxy(params map[string]any) (map[string]any, error) {
	body, err := utils.PostJSONRequest(config.Media.RESTFUL+"/index/api/addStreamPusherProxy", params)
	if err != nil {
		logrus.Errorln("ZlmAddStreamPusherProxy failed, err=", err)
		return nil, err
	}
	resp := map[string]any{}
	err = utils.JSONDecode(body, &resp)
	if err != nil {
		logrus.Errorln("ZlmAddStreamPusherProxy: json decode failed, err=", err)
		return nil, err
	}
	if _, ok := resp["code"]; !ok {
		logrus.Errorln("ZlmAddStreamPusherProxy: response error")
		return nil, errors.New("ZlmAddStreamPusherProxy: response error")
	}
	logrus.Infof("invoke api ZlmAddStreamPusherProxy response:\n%s", resp)
	return resp, nil
}

// 功能：创建GB28181 RTP接收端口，如果该端口接收数据超时，则会自动被回收(不用调用closeRtpServer接口)
//
// 范例：http://127.0.0.1/index/api/openRtpServer?port=0&tcp_mode=1&stream_id=test
func ZlmCloseRtpServer(params map[string]any) (map[string]any, error) {
	body, err := utils.PostJSONRequest(config.Media.RESTFUL+"/index/api/closeRtpServer", params)
	if err != nil {
		logrus.Errorln("ZlmCloseRtpServer failed, err=", err)
		return nil, err
	}
	resp := map[string]any{}
	err = utils.JSONDecode(body, &resp)
	if err != nil {
		logrus.Errorln("ZlmCloseRtpServer: json decode failed, err=", err)
		return nil, err
	}
	if _, ok := resp["code"]; !ok {
		logrus.Errorln("ZlmCloseRtpServer: response error")
		return nil, errors.New("ZlmCloseRtpServer: response error")
	}
	logrus.Infof("invoke api ZlmCloseRtpServer response:\n%s", resp)
	return resp, nil
}

// 功能：关闭流(目前所有类型的流都支持关闭)
//
// 范例：http://127.0.0.1/index/api/close_streams?schema=rtmp&vhost=__defaultVhost__&app=live&stream=0&force=1
func ZlmCloseStreams(params map[string]any) (map[string]any, error) {
	body, err := utils.PostJSONRequest(config.Media.RESTFUL+"/index/api/close_streams", params)
	if err != nil {
		logrus.Errorln("ZlmCloseStreams failed, err=", err)
		return nil, err
	}
	resp := map[string]any{}
	err = utils.JSONDecode(body, &resp)
	if err != nil {
		logrus.Errorln("ZlmCloseStreams: json decode failed, err=", err)
		return nil, err
	}
	if _, ok := resp["code"]; !ok {
		logrus.Errorln("ZlmCloseStreams: response error")
		return nil, errors.New("ZlmCloseStreams: response error")
	}
	logrus.Infof("invoke api ZlmCloseStreams response:\n%s", resp)
	return resp, nil
}

// 功能：关闭ffmpeg拉流代理
//
// 范例：http://127.0.0.1/index/api/delFFmpegSource?key=5f748d2ef9712e4b2f6f970c1d44d93a
func ZlmConnectRtpServer(params map[string]any) (map[string]any, error) {
	body, err := utils.PostJSONRequest(config.Media.RESTFUL+"/index/api/connectRtpServer", params)
	if err != nil {
		logrus.Errorln("ZlmConnectRtpServer failed, err=", err)
		return nil, err
	}
	resp := map[string]any{}
	err = utils.JSONDecode(body, &resp)
	if err != nil {
		logrus.Errorln("ZlmConnectRtpServer: json decode failed, err=", err)
		return nil, err
	}
	if _, ok := resp["code"]; !ok {
		logrus.Errorln("ZlmConnectRtpServer: response error")
		return nil, errors.New("ZlmConnectRtpServer: response error")
	}
	logrus.Infof("invoke api ZlmConnectRtpServer response:\n%s", resp)
	return resp, nil
}

// 功能：关闭ffmpeg拉流代理
//
// 范例：http://127.0.0.1/index/api/delFFmpegSource?key=5f748d2ef9712e4b2f6f970c1d44d93a
func ZlmDelFFmpegSource(params map[string]any) (map[string]any, error) {
	body, err := utils.PostJSONRequest(config.Media.RESTFUL+"/index/api/delFFmpegSource", params)
	if err != nil {
		logrus.Errorln("ZlmDelFFmpegSource failed, err=", err)
		return nil, err
	}
	resp := map[string]any{}
	err = utils.JSONDecode(body, &resp)
	if err != nil {
		logrus.Errorln("ZlmDelFFmpegSource: json decode failed, err=", err)
		return nil, err
	}
	if _, ok := resp["code"]; !ok {
		logrus.Errorln("ZlmDelFFmpegSource: response error")
		return nil, errors.New("ZlmDelFFmpegSource: response error")
	}
	logrus.Infof("invoke api ZlmDelFFmpegSource response:\n%s", resp)
	return resp, nil
}

// 功能：关闭拉流代理(流注册成功后，也可以使用close_streams接口替代)
//
// 范例：http://127.0.0.1/index/api/delStreamProxy?key=__defaultVhost__/proxy/0
func ZlmDelStreamProxye(params map[string]any) (map[string]any, error) {
	body, err := utils.PostJSONRequest(config.Media.RESTFUL+"/index/api/delStreamProxy", params)
	if err != nil {
		logrus.Errorln("ZlmDelStreamProxye failed, err=", err)
		return nil, err
	}
	resp := map[string]any{}
	err = utils.JSONDecode(body, &resp)
	if err != nil {
		logrus.Errorln("ZlmDelStreamProxye: json decode failed, err=", err)
		return nil, err
	}
	if _, ok := resp["code"]; !ok {
		logrus.Errorln("ZlmDelStreamProxye: response error")
		return nil, errors.New("ZlmDelStreamProxye: response error")
	}
	logrus.Infof("invoke api ZlmDelStreamProxye response:\n%s", resp)
	return resp, nil
}

// 功能：关闭推流
//
// 范例：http://127.0.0.1/index/api/delStreamPusherProxy?key=rtmp/defaultVhost/proxy/test/4AB43C9EABEB76AB443BB8260C8B2D12
func ZlmDelStreamPusherProxy(params map[string]any) (map[string]any, error) {
	body, err := utils.PostJSONRequest(config.Media.RESTFUL+"/index/api/delStreamPusherProxy", params)
	if err != nil {
		logrus.Errorln("ZlmDelStreamPusherProxy failed, err=", err)
		return nil, err
	}
	resp := map[string]any{}
	err = utils.JSONDecode(body, &resp)
	if err != nil {
		logrus.Errorln("ZlmDelStreamPusherProxy: json decode failed, err=", err)
		return nil, err
	}
	if _, ok := resp["code"]; !ok {
		logrus.Errorln("ZlmDelStreamPusherProxy: response error")
		return nil, errors.New("ZlmDelStreamPusherProxy: response error")
	}
	logrus.Infof("invoke api ZlmDelStreamPusherProxy response:\n%s", resp)
	return resp, nil
}

// 功能：获取所有TcpSession列表(获取所有tcp客户端相关信息)
//
// 范例：http://127.0.0.1/index/api/getAllSession
func ZlmGetAllSession(params map[string]any) (map[string]any, error) {
	body, err := utils.PostJSONRequest(config.Media.RESTFUL+"/index/api/getAllSession", params)
	if err != nil {
		logrus.Errorln("ZlmGetAllSession failed, err=", err)
		return nil, err
	}
	resp := map[string]any{}
	err = utils.JSONDecode(body, &resp)
	if err != nil {
		logrus.Errorln("ZlmGetAllSession: json decode failed, err=", err)
		return nil, err
	}
	if _, ok := resp["code"]; !ok {
		logrus.Errorln("ZlmGetAllSession: response error")
		return nil, errors.New("ZlmGetAllSession: response error")
	}
	logrus.Infof("invoke api ZlmGetAllSession response:\n%s", resp)
	return resp, nil
}

// 功能：获取API列表
//
// 范例：http://127.0.0.1/index/api/getApiList
func ZlmGetApiList(params map[string]any) (map[string]any, error) {
	body, err := utils.PostJSONRequest(config.Media.RESTFUL+"/index/api/getApiList", params)
	if err != nil {
		logrus.Errorln("ZlmGetApiList failed, err=", err)
		return nil, err
	}
	resp := map[string]any{}
	err = utils.JSONDecode(body, &resp)
	if err != nil {
		logrus.Errorln("ZlmGetApiList: json decode failed, err=", err)
		return nil, err
	}
	if _, ok := resp["code"]; !ok {
		logrus.Errorln("ZlmGetApiList: response error")
		return nil, errors.New("ZlmGetApiList: response error")
	}
	logrus.Infof("invoke api ZlmGetApiList response:\n%s", resp)
	return resp, nil
}

func ZlmGetMediaPlayerList(params map[string]any) (map[string]any, error) {
	body, err := utils.PostJSONRequest(config.Media.RESTFUL+"/index/api/getMediaPlayerList", params)
	if err != nil {
		logrus.Errorln("ZlmGetMediaPlayerList failed, err=", err)
		return nil, err
	}
	resp := map[string]any{}
	err = utils.JSONDecode(body, &resp)
	if err != nil {
		logrus.Errorln("ZlmGetMediaPlayerList: json decode failed, err=", err)
		return nil, err
	}
	if _, ok := resp["code"]; !ok {
		logrus.Errorln("ZlmGetMediaPlayerList: response error")
		return nil, errors.New("set config: response error")
	}
	logrus.Infof("invoke api ZlmGetMediaPlayerList response:\n%s", resp)
	return resp, nil
}

// 功能：搜索文件系统，获取流对应的录像文件列表或日期文件夹列表
//
// 范例：http://127.0.0.1/index/api/getMp4RecordFile?vhost=__defaultVhost__&app=live&stream=ss&period=2020-01
func ZlmGetMp4RecordFile(params map[string]any) (map[string]any, error) {
	body, err := utils.PostJSONRequest(config.Media.RESTFUL+"/index/api/getMp4RecordFile", params)
	if err != nil {
		logrus.Errorln("ZlmGetMp4RecordFile failed, err=", err)
		return nil, err
	}
	resp := map[string]any{}
	err = utils.JSONDecode(body, &resp)
	if err != nil {
		logrus.Errorln("ZlmGetMp4RecordFile: json decode failed, err=", err)
		return nil, err
	}
	if _, ok := resp["code"]; !ok {
		logrus.Errorln("ZlmGetMp4RecordFile: response error")
		return nil, errors.New("ZlmGetMp4RecordFile: response error")
	}
	logrus.Infof("invoke api ZlmGetMp4RecordFile response:\n%s", resp)
	return resp, nil
}

// 功能：获取服务器配置
//
// 范例：http://127.0.0.1/index/api/getServerConfig
func ZlmGetServerConfig(params map[string]any) (map[string]any, error) {
	body, err := utils.PostJSONRequest(config.Media.RESTFUL+"/index/api/getServerConfig", params)
	if err != nil {
		logrus.Errorln("ZlmGetServerConfig failed, err=", err)
		return nil, err
	}
	resp := map[string]any{}
	err = utils.JSONDecode(body, &resp)
	if err != nil {
		logrus.Errorln("ZlmGetServerConfig: json decode failed, err=", err)
		return nil, err
	}
	if _, ok := resp["code"]; !ok {
		logrus.Errorln("ZlmGetServerConfig: response error")
		return nil, errors.New("ZlmGetServerConfig: response error")
	}
	logrus.Infof("invoke api ZlmGetServerConfig response:\n%s", resp)
	return resp, nil
}

// 功能：获取截图或生成实时截图并返回
//
// 范例：http://127.0.0.1/index/api/getSnap?url=rtmp://127.0.0.1/record/robot.mp4&timeout_sec=10&expire_sec=30
func ZlmGetSnap(params map[string]any) (map[string]any, error) {
	body, err := utils.PostJSONRequest(config.Media.RESTFUL+"/index/api/getSnap", params)
	if err != nil {
		logrus.Errorln("ZlmGetSnap failed, err=", err)
		return nil, err
	}
	resp := map[string]any{}
	err = utils.JSONDecode(body, &resp)
	if err != nil {
		logrus.Errorln("ZlmGetSnap: json decode failed, err=", err)
		return nil, err
	}
	if _, ok := resp["code"]; !ok {
		logrus.Errorln("ZlmGetSnap: response error")
		return nil, errors.New("ZlmGetSnap: response error")
	}
	logrus.Infof("invoke api ZlmGetSnap response:\n%s", resp)
	return resp, nil
}

// 功能：获取主要对象个数统计，主要用于分析内存性能
//
// 范例：http://127.0.0.1/index/api/getStatistic?secret=035c73f7-bb6b-4889-a715-d9eb2d1925cc
func ZlmGetStatistic(params map[string]any) (map[string]any, error) {
	body, err := utils.PostJSONRequest(config.Media.RESTFUL+"/index/api/getStatistic", params)
	if err != nil {
		logrus.Errorln("ZlmGetStatistic failed, err=", err)
		return nil, err
	}
	resp := map[string]any{}
	err = utils.JSONDecode(body, &resp)
	if err != nil {
		logrus.Errorln("ZlmGetStatistic: json decode failed, err=", err)
		return nil, err
	}
	if _, ok := resp["code"]; !ok {
		logrus.Errorln("ZlmGetStatistic: response error")
		return nil, errors.New("ZlmGetStatistic: response error")
	}
	logrus.Infof("invoke api ZlmGetStatistic response:\n%s", resp)
	return resp, nil
}

// 功能：获取各epoll(或select)线程负载以及延时
//
// 范例：http://127.0.0.1/index/api/getThreadsLoad
func ZlmGetThreadsLoad(params map[string]any) (map[string]any, error) {
	body, err := utils.PostJSONRequest(config.Media.RESTFUL+"/index/api/getThreadsLoad", params)
	if err != nil {
		logrus.Errorln("ZlmGetThreadsLoad failed, err=", err)
		return nil, err
	}
	resp := map[string]any{}
	err = utils.JSONDecode(body, &resp)
	if err != nil {
		logrus.Errorln("ZlmGetThreadsLoad: json decode failed, err=", err)
		return nil, err
	}
	if _, ok := resp["code"]; !ok {
		logrus.Errorln("ZlmGetThreadsLoad: response error")
		return nil, errors.New("ZlmGetThreadsLoad: response error")
	}
	logrus.Infof("invoke api ZlmGetThreadsLoad response:\n%s", resp)
	return resp, nil
}

// 功能：获取各后台epoll(或select)线程负载以及延时
//
// 范例：http://127.0.0.1/index/api/getWorkThreadsLoad
func ZlmGetWorkThreadsLoad(params map[string]any) (map[string]any, error) {
	body, err := utils.PostJSONRequest(config.Media.RESTFUL+"/index/api/getWorkThreadsLoad", params)
	if err != nil {
		logrus.Errorln("ZlmGetWorkThreadsLoad failed, err=", err)
		return nil, err
	}
	resp := map[string]any{}
	err = utils.JSONDecode(body, &resp)
	if err != nil {
		logrus.Errorln("ZlmGetWorkThreadsLoad: json decode failed, err=", err)
		return nil, err
	}
	if _, ok := resp["code"]; !ok {
		logrus.Errorln("ZlmGetWorkThreadsLoad: response error")
		return nil, errors.New("ZlmGetWorkThreadsLoad: response error")
	}
	logrus.Infof("invoke api ZlmGetWorkThreadsLoad response:\n%s", resp)
	return resp, nil
}

// 功能：判断直播流是否在线(已过期，请使用getMediaList接口替代)
//
// 范例：http://127.0.0.1/index/api/isMediaOnline?schema=rtsp&vhost=__defaultVhost__&app=live&stream=obs
func ZlmIsMediaOnline(params map[string]any) (map[string]any, error) {
	body, err := utils.PostJSONRequest(config.Media.RESTFUL+"/index/api/isMediaOnline", params)
	if err != nil {
		logrus.Errorln("ZlmIsMediaOnline failed, err=", err)
		return nil, err
	}
	resp := map[string]any{}
	err = utils.JSONDecode(body, &resp)
	if err != nil {
		logrus.Errorln("ZlmIsMediaOnline: json decode failed, err=", err)
		return nil, err
	}
	if _, ok := resp["code"]; !ok {
		logrus.Errorln("ZlmIsMediaOnline: response error")
		return nil, errors.New("ZlmIsMediaOnline: response error")
	}
	logrus.Infof("invoke api ZlmIsMediaOnline response:\n%s", resp)
	return resp, nil
}

// 功能：获取流录制状态
//
// 范例：http://127.0.0.1/index/api/isRecording?type=1&vhost=__defaultVhost__&app=live&stream=obs
func ZlmIsRecording(params map[string]any) (map[string]any, error) {
	body, err := utils.PostJSONRequest(config.Media.RESTFUL+"/index/api/isRecording", params)
	if err != nil {
		logrus.Errorln("ZlmIsRecording, err=", err)
		return nil, err
	}
	resp := map[string]any{}
	err = utils.JSONDecode(body, &resp)
	if err != nil {
		logrus.Errorln("ZlmIsRecording: json decode failed, err=", err)
		return nil, err
	}
	if _, ok := resp["code"]; !ok {
		logrus.Errorln("ZlmIsRecording: response error")
		return nil, errors.New("ZlmIsRecording: response error")
	}
	logrus.Infof("invoke api ZlmIsRecording response:\n%s", resp)
	return resp, nil
}

// 功能：断开tcp连接，比如说可以断开rtsp、rtmp播放器等
//
// 范例：http://127.0.0.1/index/api/kick_session?id=140614440178720
func ZlmKickSession(params map[string]any) (map[string]any, error) {
	body, err := utils.PostJSONRequest(config.Media.RESTFUL+"/index/api/kick_session", params)
	if err != nil {
		logrus.Errorln("ZlmKickSession failed, err=", err)
		return nil, err
	}
	resp := map[string]any{}
	err = utils.JSONDecode(body, &resp)
	if err != nil {
		logrus.Errorln("ZlmKickSession: json decode failed, err=", err)
		return nil, err
	}
	if _, ok := resp["code"]; !ok {
		logrus.Errorln("ZlmKickSession: response error")
		return nil, errors.New("ZlmKickSession: response error")
	}
	logrus.Infof("invoke api ZlmKickSession response:\n%s", resp)
	return resp, nil
}

// 功能：断开tcp连接，比如说可以断开rtsp、rtmp播放器等
//
// 范例：http://127.0.0.1/index/api/kick_sessions?local_port=554
func ZlmKickSessions(params map[string]any) (map[string]any, error) {
	body, err := utils.PostJSONRequest(config.Media.RESTFUL+"/index/api/kick_sessions", params)
	if err != nil {
		logrus.Errorln("ZlmKickSessions failed, err=", err)
		return nil, err
	}
	resp := map[string]any{}
	err = utils.JSONDecode(body, &resp)
	if err != nil {
		logrus.Errorln("ZlmKickSessions: json decode failed, err=", err)
		return nil, err
	}
	if _, ok := resp["code"]; !ok {
		logrus.Errorln("ZlmKickSessions: response error")
		return nil, errors.New("ZlmKickSessions: response error")
	}
	logrus.Infof("invoke api ZlmKickSessions response:\n%s", resp)
	return resp, nil
}

// 功能：获取openRtpServer接口创建的所有RTP服务器
//
// 范例：http://127.0.0.1/index/api/listRtpServer
func ZlmListRtpServer(params map[string]any) (map[string]any, error) {
	body, err := utils.PostJSONRequest(config.Media.RESTFUL+"/index/api/listRtpServer", params)
	if err != nil {
		logrus.Errorln("ZlmListRtpServer failed, err=", err)
		return nil, err
	}
	resp := map[string]any{}
	err = utils.JSONDecode(body, &resp)
	if err != nil {
		logrus.Errorln("ZlmListRtpServer: json decode failed, err=", err)
		return nil, err
	}
	if _, ok := resp["code"]; !ok {
		logrus.Errorln("ZlmListRtpServer: response error")
		return nil, errors.New("ZlmListRtpServer: response error")
	}
	logrus.Infof("invoke api ZlmListRtpServer response:\n%s", resp)
	return resp, nil
}

// 功能：创建GB28181 RTP接收端口，如果该端口接收数据超时，则会自动被回收(不用调用closeRtpServer接口)
//
// 范例：http://127.0.0.1/index/api/openRtpServer?port=0&tcp_mode=1&stream_id=test
func ZlmOpenRtpServer(params map[string]any) (map[string]any, error) {
	body, err := utils.PostJSONRequest(config.Media.RESTFUL+"/index/api/openRtpServer", params)
	if err != nil {
		logrus.Errorln("ZlmOpenRtpServer failed, err=", err)
		return nil, err
	}
	resp := map[string]any{}
	err = utils.JSONDecode(body, &resp)
	if err != nil {
		logrus.Errorln("ZlmOpenRtpServer: json decode failed, err=", err)
		return nil, err
	}
	if _, ok := resp["code"]; !ok {
		logrus.Errorln("ZlmOpenRtpServer: response error")
		return nil, errors.New("ZlmOpenRtpServer: response error")
	}
	logrus.Infof("invoke api ZlmOpenRtpServer response:\n%s", resp)
	return resp, nil
}

// 功能：重启服务器,只有Daemon方式才能重启，否则是直接关闭！
//
// 范例：http://127.0.0.1/index/api/restartServer
func ZlmRestartServer(params map[string]any) (map[string]any, error) {
	body, err := utils.PostJSONRequest(config.Media.RESTFUL+"/index/api/restartServer", params)
	if err != nil {
		logrus.Errorln("ZlmRestartServer failed, err=", err)
		return nil, err
	}
	resp := map[string]any{}
	err = utils.JSONDecode(body, &resp)
	if err != nil {
		logrus.Errorln("ZlmRestartServer: json decode failed, err=", err)
		return nil, err
	}
	if _, ok := resp["code"]; !ok {
		logrus.Errorln("ZlmRestartServer: response error")
		return nil, errors.New("ZlmRestartServer: response error")
	}
	logrus.Infof("ZlmRestartServer response:\n%s", resp)
	return resp, nil
}

// 功能：作为GB28181客户端，启动ps-rtp推流，支持rtp/udp方式；该接口支持rtsp/rtmp等协议转ps-rtp推流。第一次推流失败会直接返回错误，成功一次后，后续失败也将无限重试。
//
// 范例：http://127.0.0.1/index/api/startSendRtp?secret=035c73f7-bb6b-4889-a715-d9eb2d1925cc&vhost=__defaultVhost__&app=live&stream=test&ssrc=1&dst_url=127.0.0.1&dst_port=10000&is_udp=0
func ZlmStartSendRtp(params map[string]any) (map[string]any, error) {
	body, err := utils.PostJSONRequest(config.Media.RESTFUL+"/index/api/startSendRtp", params)
	if err != nil {
		logrus.Errorln("ZlmStartSendRtp failed, err=", err)
		return nil, err
	}
	resp := map[string]any{}
	err = utils.JSONDecode(body, &resp)
	if err != nil {
		logrus.Errorln("ZlmStartSendRtp: json decode failed, err=", err)
		return nil, err
	}
	if _, ok := resp["code"]; !ok {
		logrus.Errorln("ZlmStartSendRtp: response error")
		return nil, errors.New("ZlmStartSendRtp: response error")
	}
	logrus.Infof("invoke api ZlmStartSendRtp response:\n%s", resp)
	return resp, nil
}

// 功能：作为GB28181 Passive TCP服务器；该接口支持rtsp/rtmp等协议转ps-rtp被动推流。调用该接口，zlm会启动tcp服务器等待连接请求，连接建立后，zlm会关闭tcp服务器，然后源源不断的往客户端推流。第一次推流失败会直接返回错误，成功一次后，后续失败也将无限重试(不停地建立tcp监听，超时后再关闭)。
//
// 范例：http://127.0.0.1/index/api/startSendRtpPassive?secret=035c73f7-bb6b-4889-a715-d9eb2d1925cc&vhost=__defaultVhost__&app=live&stream=test&ssrc=1
func ZlmStartSendRtpPassive(params map[string]any) (map[string]any, error) {
	body, err := utils.PostJSONRequest(config.Media.RESTFUL+"/index/api/startSendRtpPassive", params)
	if err != nil {
		logrus.Errorln("ZlmStartSendRtpPassive failed, err=", err)
		return nil, err
	}
	resp := map[string]any{}
	err = utils.JSONDecode(body, &resp)
	if err != nil {
		logrus.Errorln("ZlmStartSendRtpPassive: json decode failed, err=", err)
		return nil, err
	}
	if _, ok := resp["code"]; !ok {
		logrus.Errorln("ZlmStartSendRtpPassive: response error")
		return nil, errors.New("ZlmStartSendRtpPassive: response error")
	}
	logrus.Infof("invoke api ZlmStartSendRtpPassive response:\n%s", resp)
	return resp, nil
}

// 功能：停止GB28181 ps-rtp推流
//
// 范例：http://127.0.0.1/index/api/stopSendRtp?secret=035c73f7-bb6b-4889-a715-d9eb2d1925cc&vhost=__defaultVhost__&app=live&stream=test
func ZlmStopSendRtp(params map[string]any) (map[string]any, error) {
	body, err := utils.PostJSONRequest(config.Media.RESTFUL+"/index/api/stopSendRtp", params)
	if err != nil {
		logrus.Errorln("ZlmStopSendRtp failed, err=", err)
		return nil, err
	}
	resp := map[string]any{}
	err = utils.JSONDecode(body, &resp)
	if err != nil {
		logrus.Errorln("ZlmStopSendRtp: json decode failed, err=", err)
		return nil, err
	}
	if _, ok := resp["code"]; !ok {
		logrus.Errorln("ZlmStopSendRtp: response error")
		return nil, errors.New("ZlmStopSendRtp: response error")
	}
	logrus.Infof("invoke api ZlmStopSendRtp response:\n%s", resp)
	return resp, nil
}

// 功能：获取版本信息，如分支，commit id, 编译时间
//
// 范例：http://127.0.0.1/index/api/version
func ZlmVersion(params map[string]any) (map[string]any, error) {
	body, err := utils.PostJSONRequest(config.Media.RESTFUL+"/index/api/version", params)
	if err != nil {
		logrus.Errorln("ZlmVersion failed, err=", err)
		return nil, err
	}
	resp := map[string]any{}
	err = utils.JSONDecode(body, &resp)
	if err != nil {
		logrus.Errorln("ZlmVersion: json decode failed, err=", err)
		return nil, err
	}
	if _, ok := resp["code"]; !ok {
		logrus.Errorln("ZlmVersion: response error")
		return nil, errors.New("ZlmVersion: response error")
	}
	logrus.Infof("invoke api ZlmVersion response:\n%s", resp)
	return resp, nil
}

// 功能：设置zlm配置
//
// 范例：http://127.0.0.1/index/api/setServerConfig?api.apiDebug=0(例如关闭http api调试)
func ZlmSetServerConfig(params map[string]any) (map[string]any, error) {
	body, err := utils.PostJSONRequest(config.Media.RESTFUL+"/index/api/setServerConfig", params)
	if err != nil {
		logrus.Errorln("ZlmSetServerConfig failed, err=", err)
		return nil, err
	}
	resp := map[string]any{}
	err = utils.JSONDecode(body, &resp)
	if err != nil {
		logrus.Errorln("ZlmSetServerConfig: json decode failed, err=", err)
		return nil, err
	}
	if _, ok := resp["code"]; !ok {
		logrus.Errorln("ZlmSetServerConfig: response error")
		return nil, errors.New("ZlmSetServerConfig: response error")
	}
	logrus.Infof("ZlmSetServerConfig response:\n%s", resp)
	return resp, nil
}

// 功能: 同步hook到zlm配置文件
func syncWebhook2ZlmConfig() (map[string]any, error) {
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
		logrus.Errorln("syncWebhook2ZlmConfig failed, err=", err)
		return nil, err
	}
	resp := map[string]any{}
	err = utils.JSONDecode(body, &resp)
	if err != nil {
		logrus.Errorln("syncWebhook2ZlmConfig: json decode failed, err=", err)
		return nil, err
	}
	if _, ok := resp["code"]; !ok {
		logrus.Errorln("syncWebhook2ZlmConfig: response error")
		return nil, errors.New("syncWebhook2ZlmConfig: response error")
	}
	logrus.Infof("syncWebhook2ZlmConfig response:\n%s", resp)
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
		resp := map[string]any{}
		err = utils.JSONDecode(body, &resp)
		if err != nil {
			logrus.Errorln("set config: json decode failed, err=", err)
			continue
		}
		if _, ok := resp["code"]; !ok  {
			logrus.Errorln("set config: response error")
			continue
		}
	}
	return nil
}*/

func ZlmGetMediaList(params map[string]any) (map[string]any, error) {
	body, err := utils.PostJSONRequest(config.Media.RESTFUL+"/index/api/getMediaList", params)
	if err != nil {
		logrus.Errorln("ZlmGetMediaList failed, err=", err)
		return nil, err
	}
	resp := map[string]any{}
	err = utils.JSONDecode(body, &resp)
	if err != nil {
		logrus.Errorln("ZlmGetMediaList: json decode failed, err=", err)
		return nil, err
	}
	if _, ok := resp["code"]; !ok {
		logrus.Errorln("ZlmGetMediaList: response error")
		return nil, errors.New("ZlmGetMediaList: response error")
	}
	logrus.Infof("ZlmGetMediaList response:\n%s", resp)
	return resp, nil
}

func ZlmGetMediaInfo(params map[string]any) (map[string]any, error) {
	body, err := utils.PostJSONRequest(config.Media.RESTFUL+"/index/api/getMediaInfo", params)
	if err != nil {
		logrus.Errorln("ZlmGetMediaInfo failed, err=", err)
		return nil, err
	}
	resp := map[string]any{}
	err = utils.JSONDecode(body, &resp)
	if err != nil {
		logrus.Errorln("ZlmGetMediaInfo: json decode failed, err=", err)
		return nil, err
	}
	if _, ok := resp["code"]; !ok {
		logrus.Errorln("ZlmGetMediaInfo: response error")
		return nil, errors.New("ZlmGetMediaInfo: response error")
	}
	logrus.Infof("ZlmGetMediaInfo response:\n%s", resp)
	return resp, nil
}

func ZlmCloseStream(params map[string]any) (map[string]any, error) {
	body, err := utils.PostJSONRequest(config.Media.RESTFUL+"/index/api/close_stream", params)
	if err != nil {
		logrus.Errorln("ZlmCloseStream failed, err=", err)
		return nil, err
	}
	resp := map[string]any{}
	err = utils.JSONDecode(body, &resp)
	if err != nil {
		logrus.Errorln("ZlmCloseStream: json decode failed, err=", err)
		return nil, err
	}
	if _, ok := resp["code"]; !ok {
		logrus.Errorln("ZlmCloseStream: response error")
		return nil, errors.New("ZlmCloseStream: response error")
	}
	logrus.Infof("ZlmCloseStream response:\n%s", resp)
	return resp, nil
}

func ZlmDelStreamProxy(params map[string]any) (map[string]any, error) {
	body, err := utils.PostJSONRequest(config.Media.RESTFUL+"/index/api/delStreamProxy", params)
	if err != nil {
		logrus.Errorln("ZlmDelStreamProxy failed, err=", err)
		return nil, err
	}
	resp := map[string]any{}
	err = utils.JSONDecode(body, &resp)
	if err != nil {
		logrus.Errorln("ZlmDelStreamProxy: json decode failed, err=", err)
		return nil, err
	}
	if _, ok := resp["code"]; !ok {
		logrus.Errorln("ZlmDelStreamProxy: response error")
		return nil, errors.New("ZlmDelStreamProxy: response error")
	}
	logrus.Infof("ZlmDelStreamProxy response:\n%s", resp)
	return resp, nil
}

func ZlmGetRtpInfo(params map[string]any) (map[string]any, error) {
	body, err := utils.PostJSONRequest(config.Media.RESTFUL+"/index/api/getRtpInfo", params)
	if err != nil {
		logrus.Errorln("ZlmGetRtpInfo failed, err=", err)
		return nil, err
	}
	resp := map[string]any{}
	err = utils.JSONDecode(body, &resp)
	if err != nil {
		logrus.Errorln("ZlmGetRtpInfo: json decode failed, err=", err)
		return nil, err
	}
	if _, ok := resp["code"]; !ok {
		logrus.Errorln("ZlmGetRtpInfo: response error")
		return nil, errors.New("ZlmGetRtpInfo: response error")
	}
	logrus.Infof("ZlmGetRtpInfo response:\n%s", resp)
	return resp, nil
}

func ZlmStartRecord(params map[string]any) (map[string]any, error) {
	body, err := utils.PostJSONRequest(config.Media.RESTFUL+"/index/api/startRecord", params)
	if err != nil {
		logrus.Errorln("ZlmStartRecord failed, err=", err)
		return nil, err
	}
	resp := map[string]any{}
	err = utils.JSONDecode(body, &resp)
	if err != nil {
		logrus.Errorln("ZlmStartRecord: json decode failed, err=", err)
		return nil, err
	}
	if _, ok := resp["code"]; !ok {
		logrus.Errorln("ZlmStartRecord: response error")
		return nil, errors.New("ZlmStartRecord: response error")
	}
	logrus.Infof("ZlmStartRecord response:\n%s", resp)
	return resp, nil
}

func ZlmStopRecord(params map[string]any) (map[string]any, error) {
	body, err := utils.PostJSONRequest(config.Media.RESTFUL+"/index/api/stopRecord", params)
	if err != nil {
		logrus.Errorln("ZlmStopRecord failed, err=", err)
		return nil, err
	}
	resp := map[string]any{}
	err = utils.JSONDecode(body, &resp)
	if err != nil {
		logrus.Errorln("ZlmStopRecord: json decode failed, err=", err)
		return nil, err
	}
	if _, ok := resp["code"]; !ok {
		logrus.Errorln("ZlmStopRecord: response error")
		return nil, errors.New("ZlmStopRecord: response error")
	}
	logrus.Infof("ZlmStopRecord response:\n%s", resp)
	return resp, nil
}

func ZlmGetRecordStatus(params map[string]any) (map[string]any, error) {
	body, err := utils.PostJSONRequest(config.Media.RESTFUL+"/index/api/getRecordStatus", params)
	if err != nil {
		logrus.Errorln("ZlmGetRecordStatus failed, err=", err)
		return nil, err
	}
	resp := map[string]any{}
	err = utils.JSONDecode(body, &resp)
	if err != nil {
		logrus.Errorln("ZlmGetRecordStatus: json decode failed, err=", err)
		return nil, err
	}
	if _, ok := resp["code"]; !ok {
		logrus.Errorln("ZlmGetRecordStatus: response error")
		return nil, errors.New("ZlmGetRecordStatus: response error")
	}
	logrus.Infof("ZlmGetRecordStatus response:\n%s", resp)
	return resp, nil
}

func ZlmDeleteRecordDirectory(params map[string]any) (map[string]any, error) {
	body, err := utils.PostJSONRequest(config.Media.RESTFUL+"/index/api/deleteRecordDirector", params)
	if err != nil {
		logrus.Errorln("ZlmDeleteRecordDirectory failed, err=", err)
		return nil, err
	}
	resp := map[string]any{}
	err = utils.JSONDecode(body, &resp)
	if err != nil {
		logrus.Errorln("ZlmDeleteRecordDirectory: json decode failed, err=", err)
		return nil, err
	}
	if _, ok := resp["code"]; !ok {
		logrus.Errorln("ZlmDeleteRecordDirectory: response error")
		return nil, errors.New("ZlmDeleteRecordDirectory: response error")
	}
	logrus.Infof("ZlmDeleteRecordDirectory response:\n%s", resp)
	return resp, nil
}

func ZlmDownloadBin(params map[string]any) (map[string]any, error) {
	body, err := utils.PostJSONRequest(config.Media.RESTFUL+"/index/api/downloadBin", params)
	if err != nil {
		logrus.Errorln("ZlmDownloadBin failed, err=", err)
		return nil, err
	}
	resp := map[string]any{}
	err = utils.JSONDecode(body, &resp)
	if err != nil {
		logrus.Errorln("ZlmDownloadBin: json decode failed, err=", err)
		return nil, err
	}
	if _, ok := resp["code"]; !ok {
		logrus.Errorln("ZlmDownloadBin: response error")
		return nil, errors.New("ZlmDownloadBin: response error")
	}
	logrus.Infof("ZlmDownloadBin response:\n%s", resp)
	return resp, nil
}

func ZlmPauseRtpCheck(params map[string]any) (map[string]any, error) {
	body, err := utils.PostJSONRequest(config.Media.RESTFUL+"/index/api/pauseRtpCheck", params)
	if err != nil {
		logrus.Errorln("ZlmPauseRtpCheck failed, err=", err)
		return nil, err
	}
	resp := map[string]any{}
	err = utils.JSONDecode(body, &resp)
	if err != nil {
		logrus.Errorln("ZlmPauseRtpCheck: json decode failed, err=", err)
		return nil, err
	}
	if _, ok := resp["code"]; !ok {
		logrus.Errorln("ZlmPauseRtpCheck: response error")
		return nil, errors.New("ZlmPauseRtpCheck: response error")
	}
	logrus.Infof("ZlmPauseRtpCheck response:\n%s", resp)
	return resp, nil
}

func ZlmResumeRtpCheck(params map[string]any) (map[string]any, error) {
	body, err := utils.PostJSONRequest(config.Media.RESTFUL+"/index/api/resumeRtpCheck", params)
	if err != nil {
		logrus.Errorln("ZlmResumeRtpCheck failed, err=", err)
		return nil, err
	}
	resp := map[string]any{}
	err = utils.JSONDecode(body, &resp)
	if err != nil {
		logrus.Errorln("ZlmResumeRtpCheck: json decode failed, err=", err)
		return nil, err
	}
	if _, ok := resp["code"]; !ok {
		logrus.Errorln("ZlmResumeRtpCheck: response error")
		return nil, errors.New("ZlmResumeRtpCheck: response error")
	}
	logrus.Infof("ZlmResumeRtpCheck response:\n%s", resp)
	return resp, nil
}

func ZlmSeekRecordStamp(params map[string]any) (map[string]any, error) {
	body, err := utils.PostJSONRequest(config.Media.RESTFUL+"/index/api/seekRecordStamp", params)
	if err != nil {
		logrus.Errorln("ZlmSeekRecordStamp failed, err=", err)
		return nil, err
	}
	resp := map[string]any{}
	err = utils.JSONDecode(body, &resp)
	if err != nil {
		logrus.Errorln("ZlmSeekRecordStamp: json decode failed, err=", err)
		return nil, err
	}
	if _, ok := resp["code"]; !ok {
		logrus.Errorln("ZlmSeekRecordStamp: response error")
		return nil, errors.New("ZlmSeekRecordStamp: response error")
	}
	logrus.Infof("ZlmSeekRecordStamp response:\n%s", resp)
	return resp, nil
}

func ZlmSetRecordSpeed(params map[string]any) (map[string]any, error) {
	body, err := utils.PostJSONRequest(config.Media.RESTFUL+"/index/api/setRecordSpeed", params)
	if err != nil {
		logrus.Errorln("ZlmSetRecordSpeed failed, err=", err)
		return nil, err
	}
	resp := map[string]any{}
	err = utils.JSONDecode(body, &resp)
	if err != nil {
		logrus.Errorln("ZlmSetRecordSpeed: json decode failed, err=", err)
		return nil, err
	}
	if _, ok := resp["code"]; !ok {
		logrus.Errorln("ZlmSetRecordSpeed: response error")
		return nil, errors.New("ZlmSetRecordSpeed: response error")
	}
	logrus.Infof("ZlmSetRecordSpeed response:\n%s", resp)
	return resp, nil
}
