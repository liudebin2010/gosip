package sip

import (
	"errors"
	"github.com/panjjo/gosip/utils"
	"github.com/sirupsen/logrus"
	"net"
	"net/http"
	"strings"
)

//var (
//	srvParser      *parser
//	LoAddr, ReAddr net.Addr
//)

// CreateUDPServer CreateUDPServer
func (s *Server) CreateCasUDPServer(raddr, laddr string) {
	lAddr, err := net.ResolveUDPAddr("udp", laddr)
	if err != nil {
		logrus.Fatalf("CreateUDPServer resolve local addr failed, addr=%s, err=%s", laddr, err.Error())
	}
	s.udpaddr = lAddr
	s.port = NewPort(lAddr.Port)
	s.host, err = utils.ResolveSelfIP()
	if err != nil {
		logrus.Fatalf("CreateUDPServer resolveip failed, addr=%s, err=%s", laddr, err.Error())
	}

	rAddr, err := net.ResolveUDPAddr("udp", raddr)
	if err != nil {
		logrus.Fatalf("CreateUDPServer resolve remote addr failed, addr=%s, err=%s", raddr, err.Error())
	}
	LoAddr = lAddr
	ReAddr = rAddr

	udp, err := net.DialUDP("udp", lAddr, rAddr)
	if err != nil {
		logrus.Fatalf("CreateUDPServer dialudp failed, laddr=%s, raddr=%s, err=%s", laddr, raddr, err.Error())
	}
	s.conn = newUDPConnection(udp)
	srvParser = newParser()
}

// ListenUDPServer ListenUDPServer
func (s *Server) ListenCasUDPServer() {

	var (
		readdr net.Addr
		num    int
		err    error
	)
	defer srvParser.stop()
	buf := make([]byte, bufferSize)
	go s.handlerListen(srvParser.out)

	for {
		num, readdr, err = s.conn.ReadFrom(buf)
		if err != nil {
			logrus.Errorf("ListenUDPServer read data failed, err=%s, data=%s", err.Error(), string(buf))
			continue
		}
		srvParser.in <- newPacket(buf[:num], readdr)
	}
}

func (s *Server) CasNewPacket(data []byte, raddr net.Addr) {
	srvParser.in <- newPacket(data, raddr)
}

// Request Request
func (s *Server) CasRequest(req *Request) (*Transaction, error) {
	viaHop, ok := req.ViaHop()
	if !ok {
		return nil, errors.New("missing required 'Via' header")
	}
	viaHop.Host = s.host.String()
	viaHop.Port = s.port
	if viaHop.Params == nil {
		viaHop.Params = NewParams().Add("branch", String{Str: GenerateBranch()})
	}
	if !viaHop.Params.Has("rport") {
		viaHop.Params.Add("rport", nil)
	}

	tx := s.mustTX(getTXKey(req))
	return tx, tx.CasRequest(req)
}

func (s *Server) CasWrite(buf []byte) (int, error) {
	num, err := s.conn.Write(buf)
	return num, err
}

func (s *Server) CasRead(buf []byte) (int, error) {
	num, err := s.conn.Read(buf)
	return num, err
}

func (s *Server) CasSipResponse(tx *Transaction) (*Response, error) {
	response := tx.GetResponse()
	if response == nil {
		return nil, utils.NewError(nil, "response timeout, tx key:", tx.Key())
	}
	if response.StatusCode() != http.StatusOK {
		return response, utils.NewError(nil, "response fail, code=", response.StatusCode(), ", reason=", response.Reason(), ", tx key:", tx.Key())
	}
	return response, nil
}

func (s *Server) GetCasRegResponse(tx *Transaction) (*Response, error) {
	response := tx.GetResponse()
	if response == nil {
		return nil, utils.NewError(nil, "response timeout, tx key:", tx.Key())
	}
	return response, nil
}

// PingHost 测试ip:port是否可以联通
func PingHost(ipaddr string) bool {
	_, err := net.Dial("tcp", ipaddr)
	if err != nil {
		b := strings.Contains(err.Error(), "connection refused")
		if b {
			return false
		}
	}
	return true
}
