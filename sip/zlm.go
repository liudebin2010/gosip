package sipapi

import (
	"fmt"
	"net/url"

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

/**
// 功能：通过fork FFmpeg进程的方式拉流代理，支持任意协议
//
// 范例：http://127.0.0.1/index/api/addFFmpegSource?src_url=http://live.hkstv.hk.lxdns.com/live/hks2/playlist.m3u8&dst_url=rtmp://127.0.0.1/live/hks2&timeout_ms=10000&ffmpeg_cmd_key=ffmpeg.cmd
func zlmAddFFmpegSource(srcUrl, dstUrl string, timeout_ms int64, enable_hls, enable_mp4 bool, ffmpeg_cmd_key string) {
	body, err := utils.GetRequest(config.Media.RESTFUL + "/index/api/addFFmpegSource?secret=" + config.Media.Secret)
}

// 功能：动态添加rtsp/rtmp/hls/http-ts/http-flv拉流代理(只支持H264/H265/aac/G711/opus负载)
// 范例：http://127.0.0.1/index/api/addStreamProxy?vhost=__defaultVhost__&app=proxy&stream=0&url=rtmp://live.hkstv.hk.lxdns.com/live/hks2
func zlmAddStreamProxy(vhost, app, stream, url string,
	retry_count, rtp_type, timeout_sec int,
	enable_hls, enable_hls_fmp4, enable_mp4, enable_rtsp, enable_rtmp, enable_ts, enable_fmp4, hls_demand, rtsp_demand, rtmp_demand, ts_demand, fmp4_demand, enable_audio, add_mute_audio bool,
	mp4_save_path string, mp4_max_second int, mp4_as_player bool, hls_save_path string, modify_stamp int, auto_close bool) {
	body, err := utils.GetRequest(config.Media.RESTFUL + "/index/api/addStreamProxy?secret=" + config.Media.Secret)
}

// 功能：添加rtsp/rtmp主动推流(把本服务器的直播流推送到其他服务器去)
//
// 范例：http://127.0.0.1/index/api/addStreamPusherProxy?vhost=__defaultVhost__&app=proxy&stream=test&dst_url=rtmp://127.0.0.1/live/test2
func zlmAddStreamPusherProxy(vhost, schema, app, stream, dst_url string, retry_count, rtp_type int, timeout_sec float32) {
	body, err := utils.GetRequest(config.Media.RESTFUL + "/index/api/addStreamPusherProxy?secret=" + config.Media.Secret)
}

// 功能：创建GB28181 RTP接收端口，如果该端口接收数据超时，则会自动被回收(不用调用closeRtpServer接口)
//
// 范例：http://127.0.0.1/index/api/openRtpServer?port=0&tcp_mode=1&stream_id=test
func zlmCloseRtpServer(port, tcp_mode int, stream_id string) {
	body, err := utils.GetRequest(config.Media.RESTFUL + "/index/api/closeRtpServer?secret=" + config.Media.Secret)
}

// 功能：关闭流(目前所有类型的流都支持关闭)
//
// 范例：http://127.0.0.1/index/api/close_streams?schema=rtmp&vhost=__defaultVhost__&app=live&stream=0&force=1
func zlmCloseStreams(schema, vhost, app, stream string, force bool) {
	body, err := utils.GetRequest(config.Media.RESTFUL + "/index/api/close_streams?secret=" + config.Media.Secret)
}

// 功能：关闭ffmpeg拉流代理
//
// 范例：http://127.0.0.1/index/api/delFFmpegSource?key=5f748d2ef9712e4b2f6f970c1d44d93a
func zlmConnectRtpServer(key string) {
	body, err := utils.GetRequest(config.Media.RESTFUL + "/index/api/connectRtpServer?secret=" + config.Media.Secret)
}

// 功能：关闭ffmpeg拉流代理
//
// 范例：http://127.0.0.1/index/api/delFFmpegSource?key=5f748d2ef9712e4b2f6f970c1d44d93a
func zlmDelFFmpegSource(key string) {
	body, err := utils.GetRequest(config.Media.RESTFUL + "/index/api/delFFmpegSource?secret=" + config.Media.Secret)
}

// 功能：关闭拉流代理(流注册成功后，也可以使用close_streams接口替代)
//
// 范例：http://127.0.0.1/index/api/delStreamProxy?key=__defaultVhost__/proxy/0
func zlmDelStreamProxye(key string) {
	body, err := utils.GetRequest(config.Media.RESTFUL + "/index/api/delStreamProxy?secret=" + config.Media.Secret)
}

// 功能：关闭推流
//
// 范例：http://127.0.0.1/index/api/delStreamPusherProxy?key=rtmp/defaultVhost/proxy/test/4AB43C9EABEB76AB443BB8260C8B2D12
func zlmDelStreamPusherProxy(key string) {
	body, err := utils.GetRequest(config.Media.RESTFUL + "/index/api/delStreamPusherProxy?secret=" + config.Media.Secret)
}

func zlmDeleteRecordDirectory() {
	body, err := utils.GetRequest(config.Media.RESTFUL + "/index/api/deleteRecordDirectory?secret=" + config.Media.Secret)
}

func zlmDownloadBin() {
	body, err := utils.GetRequest(config.Media.RESTFUL + "/index/api/downloadBin?secret=" + config.Media.Secret)
}

// 功能：获取所有TcpSession列表(获取所有tcp客户端相关信息)
//
// 范例：http://127.0.0.1/index/api/getAllSession
func zlmGetAllSession(local_port int, peer_id string) {
	body, err := utils.GetRequest(config.Media.RESTFUL + "/index/api/getAllSession?secret=" + config.Media.Secret)
}

// 功能：获取API列表
//
// 范例：http://127.0.0.1/index/api/getApiList
func zlmGetApiList() {
	body, err := utils.GetRequest(config.Media.RESTFUL + "/index/api/getApiList?secret=" + config.Media.Secret)
}

func zlmGetMediaPlayerList() {
	body, err := utils.GetRequest(config.Media.RESTFUL + "/index/api/getMediaPlayerList?secret=" + config.Media.Secret)
}

// 功能：搜索文件系统，获取流对应的录像文件列表或日期文件夹列表
//
// 范例：http://127.0.0.1/index/api/getMp4RecordFile?vhost=__defaultVhost__&app=live&stream=ss&period=2020-01
func zlmGetMp4RecordFile(vhost, app, stream, preriod, customized_path string) {
	body, err := utils.GetRequest(config.Media.RESTFUL + "/index/api/getMp4RecordFile?secret=" + config.Media.Secret)
}

// 功能：获取服务器配置
//
// 范例：http://127.0.0.1/index/api/getServerConfig
func zlmGetServerConfig() {
	body, err := utils.GetRequest(config.Media.RESTFUL + "/index/api/getServerConfig?secret=" + config.Media.Secret)
}

// 功能：获取截图或生成实时截图并返回
//
// 范例：http://127.0.0.1/index/api/getSnap?url=rtmp://127.0.0.1/record/robot.mp4&timeout_sec=10&expire_sec=30
func zlmGetSnap(url string, timeout_sec int, expire_sec int) {
	body, err := utils.GetRequest(config.Media.RESTFUL + "/index/api/getSnap?secret=" + config.Media.Secret)
}

// 功能：获取主要对象个数统计，主要用于分析内存性能
//
// 范例：http://127.0.0.1/index/api/getStatistic?secret=035c73f7-bb6b-4889-a715-d9eb2d1925cc
func zlmGetStatistic() {
	body, err := utils.GetRequest(config.Media.RESTFUL + "/index/api/getStatistic?secret=" + config.Media.Secret)
}

// 功能：获取各epoll(或select)线程负载以及延时
//
// 范例：http://127.0.0.1/index/api/getThreadsLoad
func zlmGetThreadsLoad() {
	body, err := utils.GetRequest(config.Media.RESTFUL + "/index/api/getThreadsLoad?secret=" + config.Media.Secret)
}

// 功能：获取各后台epoll(或select)线程负载以及延时
//
// 范例：http://127.0.0.1/index/api/getWorkThreadsLoad
func zlmGetWorkThreadsLoad() {
	body, err := utils.GetRequest(config.Media.RESTFUL + "/index/api/getWorkThreadsLoad?secret=" + config.Media.Secret)
}

// 功能：判断直播流是否在线(已过期，请使用getMediaList接口替代)
//
// 范例：http://127.0.0.1/index/api/isMediaOnline?schema=rtsp&vhost=__defaultVhost__&app=live&stream=obs
func zlmIsMediaOnline(schema, vhost, app, stream string) {
	body, err := utils.GetRequest(config.Media.RESTFUL + "/index/api/isMediaOnline?secret=" + config.Media.Secret)
}

//功能：获取流录制状态
//
//范例：http://127.0.0.1/index/api/isRecording?type=1&vhost=__defaultVhost__&app=live&stream=obs
func zlmIsRecording(type int,vhost,app,stream string) {
	body, err := utils.GetRequest(config.Media.RESTFUL + "/index/api/isRecording?secret=" + config.Media.Secret)
}

//功能：断开tcp连接，比如说可以断开rtsp、rtmp播放器等
//
//范例：http://127.0.0.1/index/api/kick_session?id=140614440178720
func zlmKickSession(id string) {
	body, err := utils.GetRequest(config.Media.RESTFUL + "/index/api/kick_session?secret=" + config.Media.Secret)
}

//功能：断开tcp连接，比如说可以断开rtsp、rtmp播放器等
//
//范例：http://127.0.0.1/index/api/kick_sessions?local_port=554
func zlmKickSessions(local_port int,peer_ip string) {
	body, err := utils.GetRequest(config.Media.RESTFUL + "/index/api/kick_sessions?secret=" + config.Media.Secret)
}

//功能：获取openRtpServer接口创建的所有RTP服务器
//
//范例：http://127.0.0.1/index/api/listRtpServer
func zlmListRtpServer() zlmGetMediaListResp{
	body, err := utils.GetRequest(config.Media.RESTFUL + "/index/api/listRtpServer?secret=" + config.Media.Secret)
}

//功能：创建GB28181 RTP接收端口，如果该端口接收数据超时，则会自动被回收(不用调用closeRtpServer接口)
//
//范例：http://127.0.0.1/index/api/openRtpServer?port=0&tcp_mode=1&stream_id=test
func zlmOpenRtpServer(port,tcp_mode int,stream_id string) {
	body, err := utils.GetRequest(config.Media.RESTFUL + "/index/api/openRtpServer?secret=" + config.Media.Secret)
}

func zlmPauseRtpCheck() {
	body, err := utils.GetRequest(config.Media.RESTFUL + "/index/api/pauseRtpCheck?secret=" + config.Media.Secret)
}

//功能：重启服务器,只有Daemon方式才能重启，否则是直接关闭！
//
//范例：http://127.0.0.1/index/api/restartServer
func zlmRestartServer() {
	body, err := utils.GetRequest(config.Media.RESTFUL + "/index/api/restartServer?secret=" + config.Media.Secret)
}

//
func zlmResumeRtpCheck() {
	body, err := utils.GetRequest(config.Media.RESTFUL + "/index/api/resumeRtpCheck?secret=" + config.Media.Secret)
}

func zlmSeekRecordStamp() {
	body, err := utils.GetRequest(config.Media.RESTFUL + "/index/api/seekRecordStamp?secret=" + config.Media.Secret)
}

func zlmSetRecordSpeed() {
	body, err := utils.GetRequest(config.Media.RESTFUL + "/index/api/setRecordSpeed?secret=" + config.Media.Secret)
}

//功能：设置服务器配置
//
//范例：http://127.0.0.1/index/api/setServerConfig?api.apiDebug=0(例如关闭http api调试)
func zlmSetServerConfig() {
	body, err := utils.GetRequest(config.Media.RESTFUL + "/index/api/setServerConfig?secret=" + config.Media.Secret)
}

//功能：作为GB28181客户端，启动ps-rtp推流，支持rtp/udp方式；该接口支持rtsp/rtmp等协议转ps-rtp推流。第一次推流失败会直接返回错误，成功一次后，后续失败也将无限重试。
//
//范例：http://127.0.0.1/index/api/startSendRtp?secret=035c73f7-bb6b-4889-a715-d9eb2d1925cc&vhost=__defaultVhost__&app=live&stream=test&ssrc=1&dst_url=127.0.0.1&dst_port=10000&is_udp=0
func zlmStartSendRtp(vhost,app,stream,ssrc,dst_url string,dst_port int,is_udp bool,src_port int,pt int8,use_ps int,only_audio int) {
	body, err := utils.GetRequest(config.Media.RESTFUL + "/index/api/startSendRtp?secret=" + config.Media.Secret)
}

//功能：作为GB28181 Passive TCP服务器；该接口支持rtsp/rtmp等协议转ps-rtp被动推流。调用该接口，zlm会启动tcp服务器等待连接请求，连接建立后，zlm会关闭tcp服务器，然后源源不断的往客户端推流。第一次推流失败会直接返回错误，成功一次后，后续失败也将无限重试(不停地建立tcp监听，超时后再关闭)。
//
//范例：http://127.0.0.1/index/api/startSendRtpPassive?secret=035c73f7-bb6b-4889-a715-d9eb2d1925cc&vhost=__defaultVhost__&app=live&stream=test&ssrc=1
func zlmStartSendRtpPassive(vhost,app,stream,ssrc,dst_url string,dst_port int,is_udp bool,src_port int,pt int8,use_ps int,only_audio int) {
	body, err := utils.GetRequest(config.Media.RESTFUL + "/index/api/startSendRtpPassive?secret=" + config.Media.Secret)
}

//功能：停止GB28181 ps-rtp推流
//
//范例：http://127.0.0.1/index/api/stopSendRtp?secret=035c73f7-bb6b-4889-a715-d9eb2d1925cc&vhost=__defaultVhost__&app=live&stream=test
func zlmStopSendRtp(vhost,app,stream,ssrc string) {
	body, err := utils.GetRequest(config.Media.RESTFUL + "/index/api/stopSendRtp?secret=" + config.Media.Secret)
}

//功能：获取版本信息，如分支，commit id, 编译时间
//
//范例：http://127.0.0.1/index/api/version
func zlmVersion() {
	body, err := utils.GetRequest(config.Media.RESTFUL + "/index/api/version?secret=" + config.Media.Secret)
}
*/
