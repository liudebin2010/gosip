package api

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/panjjo/gosip/db"
	"github.com/panjjo/gosip/m"
	sipapi "github.com/panjjo/gosip/sip"
	"github.com/panjjo/gosip/utils"
	"github.com/sirupsen/logrus"
)

func ZLMWebHook(c *gin.Context) {
	method := c.Param("method")
	switch method {
	case "on_flow_report":
		logrus.Infoln("on_flow_report!")
	case "on_http_access":
		// http请求鉴权，具体业务自行实现
		c.JSON(http.StatusOK, map[string]any{
			"code":   0,
			"second": 86400})
	case "on_play":
		//视频播放触发鉴权
		c.JSON(http.StatusOK, map[string]any{
			"code": 0,
			"msg":  "",
		})
	case "on_publish":
		// 推流鉴权
		c.JSON(http.StatusOK, map[string]any{
			"code":       0,
			"enableHls":  m.MConfig.Stream.HLS,
			"enableMP4":  false,
			"enableRtxp": m.MConfig.Stream.RTMP,
			"msg":        "success",
		})
	case "on_record_mp4":
		//  mp4 录制完成
		zlmRecordMp4(c)
	case "on_record_ts":
		logrus.Infoln("on_record_ts!")
	case "on_rtp_server_timeout":
		logrus.Infoln("on_rtp_server_timeout!")
	case "on_rtsp_auth":
		logrus.Infoln("on_rtsp_auth!")
	case "on_rtsp_realm":
		logrus.Infoln("on_rtsp_realm!")
	case "on_send_rtp_stopped":
		logrus.Infoln("on_send_rtp_stopped!")
	case "on_server_keepalive":
		logrus.Infoln("on_server_keepalive!")
	case "on_server_started":
		// zlm 启动，具体业务自行实现
		m.MConfig.GB28181.MediaServer = true
		c.JSON(http.StatusOK, map[string]any{
			"code": 0,
			"msg":  "success"})
	case "on_shell_login":
		logrus.Infoln("on_shell_login!")
	case "on_stream_changed":
		// 流注册和注销通知
		zlmStreamChanged(c)
	case "on_stream_none_reader":
		// 无人阅读通知 关闭流
		zlmStreamNoneReader(c)
	case "on_stream_not_found":
		// 请求播放时，流不存在时触发
		zlmStreamNotFound(c)
	default:
		c.JSON(http.StatusOK, map[string]any{
			"code": -1,
			"msg":  "body error",
		})
	}

}

type ZLMStreamChangedData struct {
	Regist bool   `json:"regist"`
	APP    string `json:"app"`
	Stream string `json:"stream"`
	Schema string `json:"schema"`
}

func zlmStreamChanged(c *gin.Context) {
	body := c.Request.Body
	defer body.Close()
	data, err := io.ReadAll(body)
	if err != nil {
		c.JSON(http.StatusOK, map[string]any{
			"code": -1,
			"msg":  "body error",
		})
		return
	}
	req := &ZLMStreamChangedData{}
	if err := utils.JSONDecode(data, &req); err != nil {
		c.JSON(http.StatusOK, map[string]any{
			"code": -1,
			"msg":  "body error",
		})
		return
	}
	ssrc := req.Stream
	if req.Regist {
		if req.Schema == "rtmp" {
			d, ok := sipapi.StreamList.Response.Load(ssrc)
			if ok {
				// 接收到流注册事件，更新ssrc数据
				params := d.(*sipapi.Streams)
				params.Stream = true
				db.Save(db.DBClient, params)
				sipapi.StreamList.Response.Store(ssrc, params)
				// 接收到流注册后进行视频流编码分析，分析出此设备对应的编码格式并保存或更新
				sipapi.SyncDevicesCodec(ssrc, params.DeviceID)
			} else {
				// ssrc不存在，关闭流
				sipapi.SipStopPlay(ssrc)
				logrus.Infoln("closeStream on_stream_changed notfound!", req.Stream)
			}
		}
	} else {
		if req.Schema == "hls" {
			//接收到流注销事件
			_, ok := sipapi.StreamList.Response.Load(ssrc)
			if ok {
				// 流还存在，注销
				sipapi.SipStopPlay(ssrc)
				logrus.Infoln("closeStream on_stream_changed cancel!", req.Stream)
			}
		}
	}
	c.JSON(http.StatusOK, map[string]any{
		"code": 0,
		"msg":  "success"})
}

type ZLMRecordMp4Data struct {
	APP       string `json:"app"`
	Stream    string `json:"stream"`
	FileName  string `json:"file_name"`
	FilePath  string `json:"file_path"`
	FileSize  int    `json:"file_size"`
	Folder    string `json:"folder"`
	StartTime int64  `json:"start_time"`
	TimeLen   int    `json:"time_len"`
	URL       string `json:"url"`
}

func zlmRecordMp4(c *gin.Context) {
	body := c.Request.Body
	defer body.Close()
	data, err := io.ReadAll(body)
	if err != nil {
		c.JSON(http.StatusOK, map[string]any{
			"code": -1,
			"msg":  "body error",
		})
		return
	}
	req := &ZLMRecordMp4Data{}
	if err := utils.JSONDecode(data, &req); err != nil {
		c.JSON(http.StatusOK, map[string]any{
			"code": -1,
			"msg":  "body error",
		})
		return
	}
	if item, ok := sipapi.RecordList.Get(req.Stream); ok {
		sipapi.RecordList.Stop(req.Stream)
		item.Down(req.URL)
		item.Resp(fmt.Sprintf("%s/%s", m.MConfig.Media.HTTP, req.URL))
	}
	c.JSON(http.StatusOK, map[string]any{
		"code": 0,
		"msg":  "success"})
}

type ZLMStreamNotFoundData struct {
	APP    string `json:"app"`
	Params string `json:"params"`
	Stream string `json:"stream"`
	Schema string `json:"schema"`
	ID     string `json:"id"`
	IP     string `json:"ip"`
	Port   int    `json:"port"`
}

func zlmStreamNotFound(c *gin.Context) {
	body := c.Request.Body
	defer body.Close()
	data, err := io.ReadAll(body)
	if err != nil {
		c.JSON(http.StatusOK, map[string]any{
			"code": -1,
			"msg":  "body error",
		})
		return
	}
	req := &ZLMStreamNotFoundData{}
	if err := utils.JSONDecode(data, &req); err != nil {
		c.JSON(http.StatusOK, map[string]any{
			"code": -1,
			"msg":  "body error",
		})
		return
	}
	ssrc := req.Stream
	if d, ok := sipapi.StreamList.Response.Load(ssrc); ok {
		params := d.(*sipapi.Streams)
		if params.Stream {
			if params.StreamType == m.StreamTypePush {
				// 存在推流记录关闭当前，重新发起推流
				sipapi.SipStopPlay(ssrc)
				logrus.Infoln("closeStream stream pushed!", req.Stream)
			} else {
				// 拉流的，重新拉流
				sipapi.SipPlay(params)
				logrus.Infoln("closeStream stream pulled!", req.Stream)
			}
		} else {
			if time.Now().Unix() > params.Ext {
				// 发送请求，但超时未接收到推流数据，关闭流
				sipapi.SipStopPlay(ssrc)
				logrus.Infoln("closeStream stream wait timeout", req.Stream)
			}
		}
	}
	c.JSON(http.StatusOK, map[string]any{
		"code": 0,
		"msg":  "success",
	})
}

type ZLMStreamNoneReaderData struct {
	APP    string `json:"app"`
	Stream string `json:"stream"`
	Schema string `json:"schema"`
}

func zlmStreamNoneReader(c *gin.Context) {
	body := c.Request.Body
	defer body.Close()
	data, err := io.ReadAll(body)
	if err != nil {
		c.JSON(http.StatusOK, map[string]any{
			"code": -1,
			"msg":  "body error",
		})
		return
	}
	req := &ZLMStreamNoneReaderData{}
	if err := utils.JSONDecode(data, &req); err != nil {
		c.JSON(http.StatusOK, map[string]any{
			"code": -1,
			"msg":  "body error",
		})
		return
	}
	sipapi.SipStopPlay(req.Stream)
	c.JSON(http.StatusOK, map[string]any{
		"code":  0,
		"close": true,
	})
	logrus.Infoln("closeStream on_stream_none_reader", req.Stream)
}

func ZLMWebAPI(c *gin.Context) {
	method := c.Param("method")
	switch method {
	case "getServerConfig":
		logrus.Infoln("getServerConfig")
		c.JSON(http.StatusOK, map[string]any{
			"code": 0,
			"msg":  "getServerConfig",
		})
	case "getAllSession":
		logrus.Infoln("getAllSession")
		c.JSON(http.StatusOK, map[string]any{
			"code": 0,
			"msg":  "getAllSession",
		})
	case "getApiList":
		logrus.Infoln("getApiList")
		c.JSON(http.StatusOK, map[string]any{
			"code": 0,
			"msg":  "getApiList",
		})
	case "getMediaList":
		logrus.Infoln("getMediaList")
		c.JSON(http.StatusOK, map[string]any{
			"code": 0,
			"msg":  "getMediaList",
		})
	case "getThreadsLoad":
		logrus.Infoln("getThreadsLoad")
		c.JSON(http.StatusOK, map[string]any{
			"code": 0,
			"msg":  "getThreadsLoad",
		})
	case "getWorkThreadsLoad":
		logrus.Infoln("getWorkThreadsLoad")
		c.JSON(http.StatusOK, map[string]any{
			"code": 0,
			"msg":  "getWorkThreadsLoad",
		})
	case "getSnap":
		logrus.Infoln("getSnap")
		c.JSON(http.StatusOK, map[string]any{
			"code": 0,
			"msg":  "getSnap",
		})
	case "getMediaInfo":
		logrus.Infoln("getMediaInfo")
		c.JSON(http.StatusOK, map[string]any{
			"code": 0,
			"msg":  "getMediaInfo",
		})
	case "restartServer":
		logrus.Infoln("restartServer")
		c.JSON(http.StatusOK, map[string]any{
			"code": 0,
			"msg":  "restartServer",
		})
	case "addFFmpegSource":
		logrus.Infoln("addFFmpegSource")
		c.JSON(http.StatusOK, map[string]any{
			"code": 0,
			"msg":  "addFFmpegSource",
		})
	case "addStreamProxy":
		logrus.Infoln("addStreamProxy")
		c.JSON(http.StatusOK, map[string]any{
			"code": 0,
			"msg":  "addStreamProxy",
		})
	case "close_stream":
		logrus.Infoln("close_stream")
		c.JSON(http.StatusOK, map[string]any{
			"code": 0,
			"msg":  "close_stream",
		})
	case "close_streams":
		logrus.Infoln("close_streams")
		c.JSON(http.StatusOK, map[string]any{
			"code": 0,
			"msg":  "close_streams",
		})
	case "delFFmpegSource":
		logrus.Infoln("delFFmpegSource")
		c.JSON(http.StatusOK, map[string]any{
			"code": 0,
			"msg":  "delFFmpegSource",
		})
	case "delStreamProxy":
		logrus.Infoln("delStreamProxy")
		c.JSON(http.StatusOK, map[string]any{
			"code": 0,
			"msg":  "delStreamProxy",
		})
	case "kick_session":
		logrus.Infoln("kick_session")
		c.JSON(http.StatusOK, map[string]any{
			"code": 0,
			"msg":  "kick_session",
		})
	case "kick_sessions":
		logrus.Infoln("kick_sessions")
		c.JSON(http.StatusOK, map[string]any{
			"code": 0,
			"msg":  "kick_sessions",
		})
	case "setServerConfig":
		logrus.Infoln("setServerConfig")
		c.JSON(http.StatusOK, map[string]any{
			"code": 0,
			"msg":  "setServerConfig",
		})
	case "isMediaOnline":
		logrus.Infoln("isMediaOnline")
		c.JSON(http.StatusOK, map[string]any{
			"code": 0,
			"msg":  "isMediaOnline",
		})
	case "getRtpInfo":
		logrus.Infoln("getRtpInfo")
		c.JSON(http.StatusOK, map[string]any{
			"code": 0,
			"msg":  "getRtpInfo",
		})
	case "getMp4RecordFile":
		logrus.Infoln("getMp4RecordFile")
		c.JSON(http.StatusOK, map[string]any{
			"code": 0,
			"msg":  "getMp4RecordFile",
		})
	case "startRecord":
		logrus.Infoln("startRecord")
		c.JSON(http.StatusOK, map[string]any{
			"code": 0,
			"msg":  "startRecord",
		})
	case "stopRecord":
		logrus.Infoln("stopRecord")
		c.JSON(http.StatusOK, map[string]any{
			"code": 0,
			"msg":  "stopRecord",
		})
	case "getRecordStatus":
		logrus.Infoln("getRecordStatus")
		c.JSON(http.StatusOK, map[string]any{
			"code": 0,
			"msg":  "getRecordStatus",
		})
	case "openRtpServer":
		logrus.Infoln("openRtpServer")
		c.JSON(http.StatusOK, map[string]any{
			"code": 0,
			"msg":  "openRtpServer",
		})
	case "closeRtpServer":
		logrus.Infoln("closeRtpServer")
		c.JSON(http.StatusOK, map[string]any{
			"code": 0,
			"msg":  "closeRtpServer",
		})
	case "listRtpServer":
		logrus.Infoln("listRtpServer")
		c.JSON(http.StatusOK, map[string]any{
			"code": 0,
			"msg":  "listRtpServer",
		})
	case "startSendRtp":
		logrus.Infoln("startSendRtp")
		c.JSON(http.StatusOK, map[string]any{
			"code": 0,
			"msg":  "startSendRtp",
		})
	case "stopSendRtp":
		logrus.Infoln("stopSendRtp")
		c.JSON(http.StatusOK, map[string]any{
			"code": 0,
			"msg":  "stopSendRtp",
		})
	case "getStatistic":
		logrus.Infoln("getStatistic")
		c.JSON(http.StatusOK, map[string]any{
			"code": 0,
			"msg":  "getStatistic",
		})
	case "addStreamPusherProxy":
		logrus.Infoln("addStreamPusherProxy")
		c.JSON(http.StatusOK, map[string]any{
			"code": 0,
			"msg":  "addStreamPusherProxy",
		})
	case "delStreamPusherProxy":
		logrus.Infoln("delStreamPusherProxy")
		c.JSON(http.StatusOK, map[string]any{
			"code": 0,
			"msg":  "delStreamPusherProxy",
		})
	case "version":
		logrus.Infoln("version")
		response, err := sipapi.ZlmVersion()
		if err != nil {
			logrus.Infoln("get zlm version err")
		}
		c.JSON(http.StatusOK, response)
	case "getMediaPlayerList":
		logrus.Infoln("getMediaPlayerList")
		c.JSON(http.StatusOK, map[string]any{
			"code": 0,
			"msg":  "getMediaPlayerList",
		})
	default:
		logrus.Infoln("default")
		c.JSON(http.StatusOK, map[string]any{
			"code": -1,
			"msg":  "body error",
		})
	}
}
