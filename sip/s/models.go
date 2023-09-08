package sip

import (
	"fmt"
	"strings"
	"time"

	"github.com/panjjo/gosip/utils"
)

// SIP默认DefaultProtocol
var DefaultProtocol = "udp"

// SIP默认DefaultSipVersion
var DefaultSipVersion = "SIP/2.0"

// Port number
type Port uint16

// NewPort NewPort
func NewPort(port int) *Port {
	newPort := Port(port)
	return &newPort
}

// Clone clone
func (port *Port) Clone() *Port {
	if port == nil {
		return nil
	}
	newPort := *port
	return &newPort
}

func (port *Port) String() string {
	if port == nil {
		return ""
	}
	return fmt.Sprintf("%d", *port)
}

// Equals Equals
func (port *Port) Equals(other interface{}) bool {
	if p, ok := other.(*Port); ok {
		return Uint16PtrEq((*uint16)(port), (*uint16)(p))
	}

	return false
}

// MaybeString  wrapper
type MaybeString interface {
	String() string
	Equals(other interface{}) bool
}

// String string
type String struct {
	Str string
}

func (str String) String() string {
	return str.Str
}

// Equals Equals
func (str String) Equals(other interface{}) bool {
	if v, ok := other.(String); ok {
		return str.Str == v.Str
	}

	return false
}

// ContentTypeSDP SDP contenttype
var ContentTypeSDP = ContentType("application/sdp")

// ContentTypeXML XML contenttype
var ContentTypeXML = ContentType("Application/MANSCDP+xml")

// ContentTypeRtsp rtsp
var ContentTypeRtsp = ContentType("Application/MANSRTSP")

var (
	// CatalogXML 获取设备列表xml样式
	CatalogXML = `<?xml version="1.0" encoding="GB2312"?>
		<Query>
		<CmdType>Catalog</CmdType>
		<SN>%d</SN>
		<DeviceID>%s</DeviceID>
		</Query>
		`
	// RecordInfoXML 获取录像文件列表xml样式
	RecordInfoXML = `<?xml version="1.0" encoding="GB2312"?>
		<Query>
		<CmdType>RecordInfo</CmdType>
		<SN>%d</SN>
		<DeviceID>%s</DeviceID>
		<StartTime>%s</StartTime>
		<EndTime>%s</EndTime>
		<Secrecy>0</Secrecy>
		<Type>time</Type>
		</Query>
		`
	// RecordDataXML 获取录像文件列表xml样式
	RecordDataXML = `<?xml version="1.0" encoding="GB2312"?>
		<Response>
		<CmdType>RecordInfo</CmdType>
		<SN>%d</SN>
		<DeviceID>%s</DeviceID>
		<Name>%s</Name>
		<SumNum>%d</SumNum>
		<RecordList Num="1">
		<Item>
		<DeviceID>%s</DeviceID>
		<Name>%s</Name>
		<FilePath>%s</FilePath>
		<Address>%s</Address>
		<StartTime>%s</StartTime>
		<EndTime>%s</EndTime>
		<Secrecy>%d</Secrecy>
		<Type>%s</Type>
		</Item>
		</RecordList>
		</Response>
		`

	// DeviceInfoXML 查询设备详情xml样式
	DeviceInfoXML = `<?xml version="1.0" encoding="GB2312"?>
		<Query>
		<CmdType>DeviceInfo</CmdType>
		<SN>%d</SN>
		<DeviceID>%s</DeviceID>
		</Query>
		`
	// DeviceControlXML 云台控制操作xml
	DeviceControlXML = `<?xml version="1.0"?>
		<Control>
		<CmdType>DeviceControl</CmdType>
		<SN>17430</SN>
		<DeviceID>%s</DeviceID>
		<PTZCmd>%s</PTZCmd>
		<Info>
		</Info>
		</Control>
		`

	// DeviceStatusXML 查询设备状态xml样式
	DeviceStatusXML = `<?xml version="1.0"?>
		<Query>
		<CmdType>DeviceStatus</CmdType>
		<SN>17430</SN>
		<DeviceID>%s</DeviceID>
		</Query>
		`

	// KeepAliveXML 心跳详情xml
	KeepAliveXML = `<?xml version="1.0" encoding="UTF-8"?>
		<Notify>
		<CmdType>Keepalive</CmdType>
		<SN>%d</SN>
		<DeviceID>%s</DeviceID>
		<Status>OK</Status>
		<Info>
		</Info>
		</Notify>
		`
	// CatalogInfoXML 设备列表详情
	CatalogDataXML = `<?xml version="1.0" encoding="GB2312" ?>
		<Response>
		<CmdType>Catalog</CmdType>
		<SN>%d</SN>
		<DeviceID>%s</DeviceID>
		<SumNum>%d</SumNum>
		<DeviceList Num="1">
		<Item>
		<DeviceID>%s</DeviceID>
		<Name>%s</Name>
		<Manufacturer>%s</Manufacturer>
		<Model>%s</Model>
		<Owner>%s</Owner>
		<CivilCode>%s</CivilCode>
		<Address>%s</Address>
		<Parental>%d</Parental>
		<ParentID>%s</ParentID>
		<RegisterWay>%d</RegisterWay>
		<Secrecy>%d</Secrecy>
		<StreamNum>%d</StreamNum>
		<IPAddress>%s</IPAddress>
		<Port>%d</Port>
		<Status>%s</Status>
		<Longitude>%s</Longitude>
		<Latitude>%s</Latitude>
		<SubCount>%d</SubCount>
		<Info>
		<PTZType>%d</PTZType>
		<DownloadSpeed>%s</DownloadSpeed>
		</Info>
		</Item>	
		</DeviceList>
		</Response>
		`
	// DeviceInfoXML 查询设备详情xml样式
	MediaStatusXML = `<?xml version="1.0" encoding="UTF-8"?>
		<Notify>
		<CmdType>MediaStatus</CmdType>
		<SN>%s</SN>
		<DeviceID>%s</DeviceID>
		<NotifyType>201</NotifyType>
		<SessionID>%s</SessionID>
		<CurrentTime>%s</CurrentTime>
		</Notify>
		`
	// RtspRange 移动视频播放位置
	RtspRange = `PLAY MANSRTSP/1.0
		CSeq: %d
		Range: npt=%d-
		`
	// RtspScale 移动视频播放位置
	RtspScale = `PLAY MANSRTSP/1.0
		CSeq: %d
		Scale: %f
		`
)

// GetDeviceInfoXML 获取设备详情指令
func GetDeviceInfoXML(id string) []byte {
	return []byte(fmt.Sprintf(DeviceInfoXML, utils.RandInt(100000, 999999), id))
}

// GetCatalogXML 获取NVR下设备列表指令
func GetCatalogXML(id string) []byte {
	return []byte(fmt.Sprintf(CatalogXML, utils.RandInt(100000, 999999), id))
}

// GetRecordInfoXML 获取录像文件列表指令
func GetRecordInfoXML(id string, sceqNo int, start, end int64) []byte {
	return []byte(fmt.Sprintf(RecordInfoXML, sceqNo, id, time.Unix(start, 0).Format("2006-01-02T15:04:05"), time.Unix(end, 0).Format("2006-01-02T15:04:05")))
}

// GetDeviceControlXML 获取NVR下云台控制指令
func GetDeviceControlXML(id, cmd string) string {
	return fmt.Sprintf(DeviceControlXML, id, cmd)
}

// GetDeviceStatusXML 获取设备状态控制指令
func GetDeviceStatusXML(id string) string {
	return fmt.Sprintf(DeviceStatusXML, id)
}

// GetKeepAliveXML 发送心跳
func GetKeepAliveXML(id string, sn int64) string {
	return fmt.Sprintf(KeepAliveXML, sn, id)
}

// RFC3261BranchMagicCookie RFC3261BranchMagicCookie
const RFC3261BranchMagicCookie = "z9hG4bK"

// GenerateBranch returns random unique branch ID.
func GenerateBranch() string {
	return strings.Join([]string{
		RFC3261BranchMagicCookie,
		utils.RandString(32),
	}, "")
}
