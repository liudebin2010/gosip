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
	var req map[string]any
	if len(data) > 0 {
		if err := utils.JSONDecode(data, &req); err != nil {
			c.JSON(http.StatusOK, map[string]any{
				"code": -1,
				"msg":  "body error",
			})
			return
		}
	}

	switch method {
	case "getApiList":
		logrus.Infoln("getApiList")
		response, err := sipapi.ZlmGetApiList(req)
		if err != nil {
			logrus.Infoln("get getApiList err")
		}
		c.JSON(http.StatusOK, response)
	case "version":
		logrus.Infoln("version")
		response, err := sipapi.ZlmVersion(req)
		if err != nil {
			logrus.Infoln("get version err")
		}
		c.JSON(http.StatusOK, response)
	case "restartServer":
		logrus.Infoln("restartServer")
		response, err := sipapi.ZlmRestartServer(req)
		if err != nil {
			logrus.Infoln("get restartServer err")
		}
		c.JSON(http.StatusOK, response)
	case "getStatistic":
		logrus.Infoln("getStatistic")
		response, err := sipapi.ZlmGetStatistic(req)
		if err != nil {
			logrus.Infoln("get getStatistic err")
		}
		c.JSON(http.StatusOK, response)
	case "downloadBin": //功能未知
		logrus.Infoln("downloadBin")
		response, err := sipapi.ZlmDownloadBin(req)
		if err != nil {
			logrus.Infoln("get downloadBin err")
		}
		c.JSON(http.StatusOK, response)
	case "getServerConfig":
		logrus.Infoln("getServerConfig")
		response, err := sipapi.ZlmGetServerConfig(req)
		if err != nil {
			logrus.Infoln("get getServerConfig err")
		}
		c.JSON(http.StatusOK, response)
	case "setServerConfig":
		logrus.Infoln("setServerConfig")
		response, err := sipapi.ZlmSetServerConfig(req)
		if err != nil {
			logrus.Infoln("get setServerConfig err")
		}
		c.JSON(http.StatusOK, response)
	case "getAllSession":
		logrus.Infoln("getAllSession")
		response, err := sipapi.ZlmGetAllSession(req)
		if err != nil {
			logrus.Infoln("get getAllSession err")
		}
		c.JSON(http.StatusOK, response)
	case "kick_session":
		logrus.Infoln("kick_session")
		response, err := sipapi.ZlmKickSession(req)
		if err != nil {
			logrus.Infoln("get kick_session err")
		}
		c.JSON(http.StatusOK, response)
	case "kick_sessions":
		logrus.Infoln("kick_sessions")
		response, err := sipapi.ZlmKickSessions(req)
		if err != nil {
			logrus.Infoln("get kick_sessions err")
		}
		c.JSON(http.StatusOK, response)
	case "getThreadsLoad":
		logrus.Infoln("getThreadsLoad")
		response, err := sipapi.ZlmGetThreadsLoad(req)
		if err != nil {
			logrus.Infoln("get getThreadsLoad err")
		}
		c.JSON(http.StatusOK, response)
	case "getWorkThreadsLoad":
		logrus.Infoln("getWorkThreadsLoad")
		response, err := sipapi.ZlmGetWorkThreadsLoad(req)
		if err != nil {
			logrus.Infoln("get getWorkThreadsLoad err")
		}
		c.JSON(http.StatusOK, response)
	case "getMediaList":
		logrus.Infoln("getMediaList")
		response, err := sipapi.ZlmGetMediaList(req)
		if err != nil {
			logrus.Infoln("get getMediaList err")
		}
		c.JSON(http.StatusOK, response)
	case "getMediaInfo":
		logrus.Infoln("getMediaInfo")
		response, err := sipapi.ZlmGetMediaInfo(req)
		if err != nil {
			logrus.Infoln("get zlm getMediaInfo err")
		}
		c.JSON(http.StatusOK, response)
	case "isMediaOnline": //已过期，替代getMediaList
		logrus.Infoln("isMediaOnline")
		response, err := sipapi.ZlmIsMediaOnline(req)
		if err != nil {
			logrus.Infoln("get isMediaOnline err")
		}
		c.JSON(http.StatusOK, response)
	case "getMediaPlayerList":
		logrus.Infoln("getMediaPlayerList")
		response, err := sipapi.ZlmGetMediaPlayerList(req)
		if err != nil {
			logrus.Infoln("get zlm getMediaPlayerList err")
		}
		c.JSON(http.StatusOK, response)
	case "addFFmpegSource":
		logrus.Infoln("addFFmpegSource")
		response, err := sipapi.ZlmAddFFmpegSource(req)
		if err != nil {
			logrus.Infoln("get addFFmpegSource err")
		}
		c.JSON(http.StatusOK, response)
	case "delFFmpegSource":
		logrus.Infoln("delFFmpegSource")
		response, err := sipapi.ZlmDelFFmpegSource(req)
		if err != nil {
			logrus.Infoln("get delFFmpegSource err")
		}
		c.JSON(http.StatusOK, response)
	case "addStreamProxy":
		logrus.Infoln("addStreamProxy")
		response, err := sipapi.ZlmAddStreamProxy(req)
		if err != nil {
			logrus.Infoln("get addStreamProxy err")
		}
		c.JSON(http.StatusOK, response)
	case "close_stream": //已过期，替代close_streams
		logrus.Infoln("close_stream")
		response, err := sipapi.ZlmCloseStream(req)
		if err != nil {
			logrus.Infoln("get close_stream err")
		}
		c.JSON(http.StatusOK, response)
	case "close_streams":
		logrus.Infoln("close_streams")
		response, err := sipapi.ZlmCloseStreams(req)
		if err != nil {
			logrus.Infoln("get close_streams err")
		}
		c.JSON(http.StatusOK, response)
	case "delStreamProxy":
		logrus.Infoln("delStreamProxy")
		response, err := sipapi.ZlmDelStreamProxy(req)
		if err != nil {
			logrus.Infoln("get delStreamProxy err")
		}
		c.JSON(http.StatusOK, response)
	case "addStreamPusherProxy":
		logrus.Infoln("addStreamPusherProxy")
		response, err := sipapi.ZlmAddStreamPusherProxy(req)
		if err != nil {
			logrus.Infoln("get addStreamPusherProxy err")
		}
		c.JSON(http.StatusOK, response)
	case "delStreamPusherProxy":
		logrus.Infoln("delStreamPusherProxy")
		response, err := sipapi.ZlmDelStreamPusherProxy(req)
		if err != nil {
			logrus.Infoln("get delStreamPusherProxy err")
		}
		c.JSON(http.StatusOK, response)
	case "getSnap":
		logrus.Infoln("getSnap")
		response, err := sipapi.ZlmGetSnap(req)
		if err != nil {
			logrus.Infoln("get getSnap err")
		}
		c.JSON(http.StatusOK, response)
	case "listRtpServer":
		logrus.Infoln("listRtpServer")
		response, err := sipapi.ZlmListRtpServer(req)
		if err != nil {
			logrus.Infoln("get listRtpServer err")
		}
		c.JSON(http.StatusOK, response)
	case "connectRtpServer":
		logrus.Infoln("connectRtpServer")
		response, err := sipapi.ZlmConnectRtpServer(req)
		if err != nil {
			logrus.Infoln("get connectRtpServer err")
		}
		c.JSON(http.StatusOK, response)
	case "openRtpServer":
		logrus.Infoln("openRtpServer")
		response, err := sipapi.ZlmOpenRtpServer(req)
		if err != nil {
			logrus.Infoln("get openRtpServer err")
		}
		c.JSON(http.StatusOK, response)
	case "closeRtpServer":
		logrus.Infoln("closeRtpServer")
		response, err := sipapi.ZlmCloseRtpServer(req)
		if err != nil {
			logrus.Infoln("get closeRtpServer err")
		}
		c.JSON(http.StatusOK, response)
	case "startSendRtp":
		logrus.Infoln("startSendRtp")
		response, err := sipapi.ZlmStartSendRtp(req)
		if err != nil {
			logrus.Infoln("get startSendRtp err")
		}
		c.JSON(http.StatusOK, response)
	case "getRtpInfo":
		logrus.Infoln("getRtpInfo")
		response, err := sipapi.ZlmGetRtpInfo(req)
		if err != nil {
			logrus.Infoln("get getRtpInfo err")
		}
		c.JSON(http.StatusOK, response)
	case "stopSendRtp":
		logrus.Infoln("stopSendRtp")
		response, err := sipapi.ZlmStopSendRtp(req)
		if err != nil {
			logrus.Infoln("get stopSendRtp err")
		}
		c.JSON(http.StatusOK, response)
	case "startSendRtpPassive":
		logrus.Infoln("startSendRtpPassive")
		response, err := sipapi.ZlmStartSendRtpPassive(req)
		if err != nil {
			logrus.Infoln("get startSendRtpPassive err")
		}
		c.JSON(http.StatusOK, response)
	case "pauseRtpCheck":
		logrus.Infoln("pauseRtpCheck")
		response, err := sipapi.ZlmPauseRtpCheck(req)
		if err != nil {
			logrus.Infoln("get pauseRtpCheck err")
		}
		c.JSON(http.StatusOK, response)
	case "resumeRtpCheck":
		logrus.Infoln("resumeRtpCheck")
		response, err := sipapi.ZlmResumeRtpCheck(req)
		if err != nil {
			logrus.Infoln("get resumeRtpCheck err")
		}
		c.JSON(http.StatusOK, response)
	case "startRecord":
		logrus.Infoln("startRecord")
		response, err := sipapi.ZlmStartRecord(req)
		if err != nil {
			logrus.Infoln("get startRecord err")
		}
		c.JSON(http.StatusOK, response)
	case "getMp4RecordFile":
		logrus.Infoln("getMp4RecordFile")
		response, err := sipapi.ZlmGetMp4RecordFile(req)
		if err != nil {
			logrus.Infoln("get getMp4RecordFile err")
		}
		c.JSON(http.StatusOK, response)
	case "getRecordStatus":
		logrus.Infoln("getRecordStatus")
		response, err := sipapi.ZlmGetRecordStatus(req)
		if err != nil {
			logrus.Infoln("get getRecordStatus err")
		}
		c.JSON(http.StatusOK, response)
	case "isRecording":
		logrus.Infoln("isRecording")
		response, err := sipapi.ZlmIsRecording(req)
		if err != nil {
			logrus.Infoln("get isRecording err")
		}
		c.JSON(http.StatusOK, response)
	case "seekRecordStamp":
		logrus.Infoln("seekRecordStamp")
		response, err := sipapi.ZlmSeekRecordStamp(req)
		if err != nil {
			logrus.Infoln("get seekRecordStamp err")
		}
		c.JSON(http.StatusOK, response)
	case "setRecordSpeed":
		logrus.Infoln("setRecordSpeed")
		response, err := sipapi.ZlmSetRecordSpeed(req)
		if err != nil {
			logrus.Infoln("get setRecordSpeed err")
		}
		c.JSON(http.StatusOK, response)
	case "stopRecord":
		logrus.Infoln("stopRecord")
		response, err := sipapi.ZlmStopRecord(req)
		if err != nil {
			logrus.Infoln("get stopRecord err")
		}
		c.JSON(http.StatusOK, response)
	case "deleteRecordDirectory":
		logrus.Infoln("deleteRecordDirectory")
		response, err := sipapi.ZlmDeleteRecordDirectory(req)
		if err != nil {
			logrus.Infoln("get deleteRecordDirectory err")
		}
		c.JSON(http.StatusOK, response)
	default:
		logrus.Infoln("default")
		c.JSON(http.StatusOK, map[string]any{
			"code": -1,
			"msg":  "body error",
		})
	}
}
