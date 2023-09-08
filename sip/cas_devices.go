package sipapi

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/panjjo/gosip/db"
	sip "github.com/panjjo/gosip/sip/s"
	"github.com/panjjo/gosip/utils"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	"strings"
	"time"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

var (
	// 全部nvr数量
	nvrCount int
	// 序列号
	catalogSN int64
)

// getCameraNum 获取所有通道数量
func getCameraNum() int64 {
	var skip, count int
	count = 100

	var dnum int64
	for {
		nvrs := []Devices{}
		// 查找活跃注册NVR设备
		db.FindT(db.DBClient, new(Devices), &nvrs, db.M{"active > ?": time.Now().Unix() - 1800, "regist=?": true}, "", skip, count, false)
		for _, nvr := range nvrs {
			num, _ := db.FindT(db.DBClient, new(Devices), &nvrs, db.M{"pdid=?": nvr.DeviceID}, "", skip, count, true)
			dnum += num
		}
		if len(nvrs) < int(count) {
			break
		}
		skip += count
	}
	return dnum
}

func sipMessageCatalogMongo(u Devices, body string) error {
	message := &MessageDeviceListResponse{}
	if err := utils.XMLDecode([]byte(body), message); err != nil {
		logrus.Errorf("send catalog message Unmarshal xml err:%s, body:\n%s", err, body)
		return err
	}

	// 创建城区街道树结构
	// sipMessageCatalogTreeMysql(message.SN)
	dnum := getCameraNum()

	var skip, count int
	count = 100
	for {
		nvrs := []Devices{}
		// 查找活跃注册NVR设备
		//_, _ = dbClient.Find(userTB, M{"regist": true, "active": M{"$gt": time.Now().Unix() - 1800}}, skip, count, "", false, &nvrs, nil)
		_, _ = db.FindT(db.DBClient, new(Devices), &nvrs, db.M{"regist": true, "active>?": time.Now().Unix() - 1800}, "", skip, count, false)
		var dskip int
		for _, nvr := range nvrs {
			devices := []DeviceItem{}
			// 查找NVR挂载相机
			//_, _ = dbClient.Find(deviceTB, M{"pdid": nvr.DeviceID}, dskip, count, "", false, &devices, nil)
			_, _ = db.FindT(db.DBClient, new(Devices), &nvrs, db.M{"pdid=?": nvr.DeviceID}, "", dskip, count, false)
			for _, d := range devices {
				status := transDeviceStatus(d.Status)
				if status != "ON" {
					continue
				}
				body := fmt.Sprintf(sip.CatalogDataXML,
					message.SN, config.GB28181.LID, dnum, d.DeviceID, d.Name, d.Manufacturer, d.Model,
					d.Owner, d.CivilCode, d.Address, d.Parental, d.ParentID, d.RegisterWay, d.Secrecy,
					d.StreamNum, d.IPAddress, d.Port, d.Status, "", "", 0, d.PTZType, d.DownloadSpeed)
				reader := transform.NewReader(bytes.NewReader([]byte(body)), simplifiedchinese.GBK.NewEncoder())
				b, _ := ioutil.ReadAll(reader)

				msgLock.Lock()
				msgSeq++
				if msgSeq > 10000 {
					msgSeq = 10
				}
				msgLock.Unlock()

				// from
				furi, _ := sip.ParseSipURI(fmt.Sprintf("sip:%s@%s", config.GB28181.LID, config.Cascade.LAddr))
				fromaddr := &sip.Address{
					URI:    &furi,
					Params: sip.NewParams(),
				}
				fromaddr.Params.Add("tag", sip.String{Str: utils.RandString(10)})
				// to
				turi, _ := sip.ParseSipURI(fmt.Sprintf("sip:%s@%s", config.Cascade.SID, config.Cascade.SRegion))
				toaddr := &sip.Address{
					URI:    &turi,
					Params: sip.NewParams(),
				}

				hb := sip.NewHeaderBuilder().SetTo(toaddr).SetFrom(fromaddr).AddVia(&sip.ViaHop{
					Params: sip.NewParams().Add("branch", sip.String{Str: sip.GenerateBranch()}),
				}).SetContentType(&sip.ContentTypeXML).SetMethod(sip.MESSAGE).SetSeqNo(msgSeq)
				req := sip.NewRequest("", sip.MESSAGE, toaddr.URI, sip.DefaultSipVersion, hb.Build(), b)
				req.SetDestination(sip.ReAddr)
				tx, err := cascadeSrv.CasRequest(req)
				if err != nil {
					logrus.Warn("catalog request error, str=", err.Error())
					continue
				}
				logrus.Infof("cas catalog request, deviceid=%s, chanid=%s", config.GB28181.LID, d.DeviceID)
				logrus.Debugf("cas catalog request str:\n%s", req.String())

				_, err = cascadeSrv.CasSipResponse(tx)
				if err != nil {
					logrus.Warnf("catalog response error, chanid=%s, err=%s", d.DeviceID, err.Error())
					continue
				}
				logrus.Info("cas catalog response finished")
			}
		}
		if len(nvrs) < int(count) {
			break
		}
		skip += count
	}
	return nil
}

// rtsp方式
type NVRLists struct {
	Code int         `json:"code"`
	Data NVRListData `json:"data"`
	Msg  string      `json:"msg"`
}

type NVRListData struct {
	List     []NVRData `json:"list"`
	Total    int       `json:"total"`
	Page     int       `json:"page"`
	PageSize int       `json:"pageSize"`
	NvrData  NVRData   `json:"renvrinfo"`
}

type NVRData struct {
	// 设备ID
	DeviceID string `json:"deviceId"`
	// 通道ID
	ChannelID   string `json:"channelId"`
	ChannelCode string `json:"channelCode"`
	// 设备类型
	DevType string `json:"devType"`
	// 所在区
	District string `json:"district"`
	// 街道
	Street string `json:"street"`
	// 停车场
	ParkName string `json:"park"`
	// NVR信息
	NVRIP      string `json:"nvrIp"`
	NVRWebPort string `json:"nvrWebPort"`
	RtspPort   string `json:"nvrRtspPort"`
	NVRUser    string `json:"nvrUser"`
	NVRPwd     string `json:"nvrPwd"`
	// 泊位ID
	Berths string `json:"berths"`
	// 经纬度坐标
	Coordinate string `json:"gcj02Coord"`
	// 内网IP
	IPAddress string
}

type DistrictLists struct {
	Code int              `json:"code"`
	Data DistrictListData `json:"data"`
	Msg  string           `json:"msg"`
}

type DistrictListData struct {
	List     []DistrictData `json:"list"`
	Total    int            `json:"total"`
	Page     int            `json:"page"`
	PageSize int            `json:"pageSize"`
}

type DistrictData struct {
	// Code 街道编码
	Code string `json:"code"`
	// Name 城区或街道名称
	Name  string `json:"name"`
	PCode string `json:"pCode"`
	Num   int    `json:"sNumber"`
}

func getNvrData(chanID string) (*NVRData, error) {
	// http://10.100.35.123:8888/nvrinfo2/findNvrInfo2?channelId=11260001062000000001
	//url := fmt.Sprintf("%s/nvrinfo2/findNvrInfo2?channelId=%s", config.DB.HTTP, chanID)
	//body, err := utils.GetRequest(url)
	//if err != nil {
	//	logrus.Error("getNvrData request nvr data failed, err=", err.Error())
	//	return nil, err
	//}

	nvrList := &NVRLists{}
	//if err = utils.JSONDecode(body, nvrList); err != nil {
	//	logrus.Error("getNvrData decode nvr data failed, err=", err.Error())
	//	return nil, err
	//}
	if nvrList.Code != 0 {
		logrus.Error("getNvrData get nvr data failed, msg=", nvrList.Msg)
		return nil, errors.New("get nvr data failed")
	}

	return &nvrList.Data.NvrData, nil
}

func getAllNvrCount() int {
	//url := fmt.Sprintf("%s/nvrinfo2/getNvrInfoList?page=%d&pageSize=%d", config.DB.HTTP, 1, 1)
	//body, err := utils.GetRequest(url)
	//if err != nil {
	//	logrus.Error("getAllNvrCount request nvr list failed, err=", err.Error())
	//	return 0
	//}
	nvrList := &NVRLists{}
	//if err = utils.JSONDecode(body, nvrList); err != nil {
	//	logrus.Error("getAllNvrCount decode nvr list failed, err=", err.Error())
	//	return 0
	//}
	if nvrList.Code != 0 {
		logrus.Error("getAllNvrCount get nvr list failed, msg=", nvrList.Msg)
		return 0
	}
	// 获取全部nvr数量
	nvrCount := nvrList.Data.Total

	//page, size := 1, 100
	//url = fmt.Sprintf("%s/street/getStreetList?page=%d&pageSize=%d&pCode=%s", config.DB.HTTP, page, size, config.Cascade.CityID)
	//body, err = utils.GetRequest(url)
	//if err != nil {
	//	logrus.Error("getAllNvrCount request district list failed, err=", err.Error())
	//	return 0
	//}

	disList := &DistrictLists{}
	//if err = utils.JSONDecode(body, disList); err != nil {
	//	logrus.Error("getAllNvrCount decode district list failed, err=", err.Error())
	//	return 0
	//}
	disCount := disList.Data.Total

	// 城区
	streetCount := 0
	for _, dist := range disList.Data.List {
		if dist.Code == "" {
			continue
		}
		//url = fmt.Sprintf("%s/street/getStreetList?page=%d&pageSize=%d&pCode=%s", config.DB.HTTP, 1, 1, dist.Code)
		//body, err := utils.GetRequest(url)
		//if err != nil {
		//	logrus.Error("getAllNvrCount request street list failed, err=", err.Error())
		//	break
		//}
		streetList := &DistrictLists{}
		//if err = utils.JSONDecode(body, streetList); err != nil {
		//	logrus.Error("getAllNvrCount decode street list failed, err=", err.Error())
		//	break
		//}
		streetCount += streetList.Data.Total
	}
	return nvrCount + disCount + streetCount
}

func sendCatalog(sn int, deviceID, channelID, name string, nvrNum, subNum int) error {
	body := fmt.Sprintf(sip.CatalogDataXML,
		sn, deviceID, nvrNum, channelID, name,
		"Manufacturer", "Camera", "Owner", "CivilCode", "",
		1, deviceID, 1, 0, 0, "", 0, "ON", "", "", subNum, 0, "")
	reader := transform.NewReader(bytes.NewReader([]byte(body)), simplifiedchinese.GBK.NewEncoder())
	b, _ := ioutil.ReadAll(reader)

	msgLock.Lock()
	msgSeq++
	if msgSeq > 10000 {
		msgSeq = 10
	}
	msgLock.Unlock()

	// from
	furi, _ := sip.ParseSipURI(fmt.Sprintf("sip:%s@%s", config.GB28181.LID, config.Cascade.LAddr))
	fromaddr := &sip.Address{
		URI:    &furi,
		Params: sip.NewParams(),
	}
	fromaddr.Params.Add("tag", sip.String{Str: utils.RandString(10)})
	// to
	turi, _ := sip.ParseSipURI(fmt.Sprintf("sip:%s@%s", config.Cascade.SID, config.Cascade.SRegion))
	toaddr := &sip.Address{
		URI:    &turi,
		Params: sip.NewParams(),
	}

	hb := sip.NewHeaderBuilder().SetTo(toaddr).SetFrom(fromaddr).AddVia(&sip.ViaHop{
		Params: sip.NewParams().Add("branch", sip.String{Str: sip.GenerateBranch()}),
	}).SetContentType(&sip.ContentTypeXML).SetMethod(sip.MESSAGE).SetSeqNo(msgSeq)
	req := sip.NewRequest("", sip.MESSAGE, toaddr.URI, sip.DefaultSipVersion, hb.Build(), b)
	req.SetDestination(sip.ReAddr)
	// tx, err := srv.Request(req)
	tx, err := cascadeSrv.CasRequest(req)
	if err != nil {
		logrus.Warnf("mysql catalog request error, err=%s", err.Error())
		return err
	}
	logrus.Debugf("mysql catalog request, pid:%s, sid:%s, name:%s", deviceID, channelID, name)
	// _, err = sip.SipResponse(tx)
	_, err = cascadeSrv.CasSipResponse(tx)
	if err != nil {
		logrus.Warnf("mysql catalog response error, sid=%s, err=%s", channelID, err.Error())
		return err
	}
	logrus.Debugf("mysql catalog response ok, sid:%s", channelID)
	return nil
}

func sipMessageCatalogCityMysql(sn int) error {
	nvrCount = getAllNvrCount() + 1 // 1为城市名称
	if nvrCount == 0 {
		nvrCount = 10000
	}

	// 发送城市名称
	// sendCatalog(sn, config.GB28181.UID, config.GB28181.CityID, config.GB28181.CityName, nvrCount, 1)
	//page, size := 1, 1000
	for {
		// http://123.57.194.199:10010/street/getStreetList?page=1&pageSize=10&pCode=11260001002150000001
		//url := fmt.Sprintf("%s/street/getStreetList?page=%d&pageSize=%d&pCode=%s", config.DB.HTTP, page, size, config.Cascade.CityID)
		//body, err := utils.GetRequest(url)
		//if err != nil {
		//	logrus.Error("sipMessageCatalogTreeMysql request district list failed, err=", err.Error())
		//	break
		//}

		disList := &DistrictLists{}
		//if err = utils.JSONDecode(body, disList); err != nil {
		//	logrus.Error("sipMessageCatalogTreeMysql decode district list failed, err=", err.Error())
		//	break
		//}
		if disList.Code != 0 {
			logrus.Error("sipMessageCatalogTreeMysql get district list failed, msg=", disList.Msg)
			break
		}

		// 城区
		for _, dist := range disList.Data.List {
			if dist.Code == "" {
				continue
			}
			// 发送城区目录
			dist.Name = strings.TrimSpace(dist.Name)
			sendCatalog(sn, config.GB28181.LID, dist.Code, dist.Name, nvrCount, dist.Num)

			//url = fmt.Sprintf("%s/street/getStreetList?page=%d&pageSize=%d&pCode=%s", config.DB.HTTP, page, size, dist.Code)
			//body, err := utils.GetRequest(url)
			//if err != nil {
			//	logrus.Error("sipMessageCatalogTreeMysql request street list failed, err=", err.Error())
			//	break
			//}
			streetList := &DistrictLists{}
			//if err = utils.JSONDecode(body, streetList); err != nil {
			//	logrus.Error("sipMessageCatalogTreeMysql decode street list failed, err=", err.Error())
			//	break
			//}

			// 街道
			for _, street := range streetList.Data.List {
				street.Name = strings.TrimSpace(street.Name)
				streetMap.Store(dist.Name+street.Name, street)
				sendCatalog(sn, dist.Code, street.Code, street.Name, nvrCount, street.Num)
			}
		}
		break
	}
	return nil
}

func sipMessageCatalogMysql() {
	// 创建城区街道树结构
	if catalogSN > 10000000 {
		catalogSN = 100000
	}
	catalogSN++
	sipMessageCatalogNvrMysql(int(catalogSN))
}

func sipMessageCatalogNvrMysql(SN int) error {

	sipMessageCatalogCityMysql(SN)
	page, size := 1, 100
	for {
		//url := fmt.Sprintf("%s/nvrinfo2/getNvrInfoList?page=%d&pageSize=%d", config.DB.HTTP, page, size)
		//body, err := utils.GetRequest(url)
		//if err != nil {
		//	logrus.Error("sipMessageCatalogNvrMysql request nvr list failed, err=", err.Error())
		//	break
		//}

		nvrList := &NVRLists{}
		//if err = utils.JSONDecode(body, nvrList); err != nil {
		//	logrus.Error("sipMessageCatalogNvrMysql decode nvr list failed, err=", err.Error())
		//	break
		//}
		if nvrList.Code != 0 {
			logrus.Error("sipMessageCatalogNvrMysql get nvr list failed, msg=", nvrList.Msg)
			break
		}

		// count = nvrList.Data.Total
		for _, device := range nvrList.Data.List {
			coor := strings.Split(device.Coordinate, ",")
			var log, lat string
			if len(coor) == 2 {
				log = coor[0]
				lat = coor[1]
			}
			channelName := device.ParkName + device.NVRWebPort + "-" + device.ChannelCode
			device.Street = strings.TrimSpace(device.Street)
			d, ok := streetMap.Load(device.District + device.Street)
			if !ok {
				logrus.Errorf("Could not find the street, n=%s", device.District+device.Street)
				continue
			}
			street := d.(DistrictData)

			body := fmt.Sprintf(sip.CatalogDataXML,
				SN, street.Code, nvrCount, device.ChannelID, channelName,
				"Manufacturer", "Camera", "Owner", "CivilCode", device.IPAddress,
				0, street.Code, 1, 0, 0, "", 0, "ON", log, lat, 0, 0, "")

			reader := transform.NewReader(bytes.NewReader([]byte(body)), simplifiedchinese.GBK.NewEncoder())
			b, _ := ioutil.ReadAll(reader)

			msgLock.Lock()
			msgSeq++
			if msgSeq > 10000 {
				msgSeq = 10
			}
			msgLock.Unlock()

			// from
			furi, _ := sip.ParseSipURI(fmt.Sprintf("sip:%s@%s", config.GB28181.LID, config.Cascade.LAddr))
			fromaddr := &sip.Address{
				URI:    &furi,
				Params: sip.NewParams(),
			}
			fromaddr.Params.Add("tag", sip.String{Str: utils.RandString(10)})
			// to
			turi, _ := sip.ParseSipURI(fmt.Sprintf("sip:%s@%s", config.Cascade.SID, config.Cascade.SRegion))
			toaddr := &sip.Address{
				URI:    &turi,
				Params: sip.NewParams(),
			}

			hb := sip.NewHeaderBuilder().SetTo(toaddr).SetFrom(fromaddr).AddVia(&sip.ViaHop{
				Params: sip.NewParams().Add("branch", sip.String{Str: sip.GenerateBranch()}),
			}).SetContentType(&sip.ContentTypeXML).SetMethod(sip.MESSAGE).SetSeqNo(msgSeq)
			req := sip.NewRequest("", sip.MESSAGE, toaddr.URI, sip.DefaultSipVersion, hb.Build(), b)
			req.SetDestination(sip.ReAddr)
			// tx, err := srv.Request(req)
			tx, err := cascadeSrv.CasRequest(req)
			if err != nil {
				logrus.Warnf("sipMessageCatalogNvrMysql catalog request error, err=%s", err.Error())
				continue
			}
			_, err = cascadeSrv.CasSipResponse(tx)
			if err != nil {
				logrus.Warnf("sipMessageCatalogNvrMysql catalog response error, sid=%s, err=%s", device.ChannelID, err.Error())
				continue
			}
		}
		if len(nvrList.Data.List) < size {
			logrus.Infof("sipMessageCatalogNvrMysql catalog num=%d", nvrList.Data.Total)
			break
		}
		page++
	}
	return nil
}
