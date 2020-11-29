package transport

import (
	"net"
	"time"

	"github.com/jart/gosip/sip"
	"github.com/qiniu/x/xlog"
)

const sipMaxPacketSize = 1500

type Transport struct {
	Conn *net.UDPConn
	Recv chan *sip.Msg
	Send chan *sip.Msg
}

func StartSip(xlog *xlog.Logger, remoteAddr string, transport string) (*Transport, error) {
	rAddr, err := net.ResolveUDPAddr("udp", remoteAddr)
	if err != nil {
		return nil, err
	}
	net, err := net.DialUDP(transport, nil, rAddr)
	if err != nil {
		xlog.Errorf("err = %#v", err.Error())
		return nil, err
	}
	recvChan := make(chan *sip.Msg)
	sendChan := make(chan *sip.Msg)
	go send(xlog, net, sendChan)
	go recv(xlog, net, recvChan)
	tr := &Transport{
		Conn: net,
		Recv: recvChan,
		Send: sendChan,
	}

	return tr, nil
}

func recv(xlog *xlog.Logger, conn *net.UDPConn, output chan *sip.Msg) {

	for {
		buf := make([]byte, sipMaxPacketSize)
		conn.SetReadDeadline(time.Now().Add(time.Second * 5))
		n, err := conn.Read(buf)
		if n == 0 || err != nil {
			continue
		}
		msg, err := sip.ParseMsg(buf[:n])
		if err != nil {
			xlog.Errorf("parse msg failed, err =%v", err)
			continue
		}
		xlog.Debug("recv msg \n", msg)
		output <- msg
	}
}

func send(xlog *xlog.Logger, conn *net.UDPConn, input chan *sip.Msg) {

	for m := range input {
		xlog.Debug("send msg \n", m)
		if _, err := conn.Write([]byte(m.String())); err != nil {
			xlog.Errorf("send msg failed, err = #v", err)
		}
	}

}
