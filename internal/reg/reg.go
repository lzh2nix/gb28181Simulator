package reg

import (
	"encoding/xml"
	"net"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/jart/gosip/sip"
	"github.com/jart/gosip/util"
	"github.com/lzh2nix/gb28181Simulator/internal/config"
	"github.com/lzh2nix/gb28181Simulator/internal/transport"
	"github.com/lzh2nix/gb28181Simulator/internal/version"
	"github.com/qiniu/x/xlog"
)

type Leg struct {
	callID  string
	FromTag string
}
type Registar struct {
	cfg *config.Config
	// device registered or not, if not resend register in regretry
	registed int32
	// reg sequence
	regSeq int32
	// last sent reginfo
	regLeg *Leg

	// send un-register  when close
	CloseChan chan bool
	// after 3 times timeout we need retry register
	keepaliveTimeoutCount int32
	keepaliveLegs         []Leg
	keepaliveSeq          int32
}

func NewRegistar(cfg *config.Config) (*Registar, error) {
	reg := &Registar{
		cfg:           cfg,
		CloseChan:     make(chan bool),
		keepaliveLegs: make([]Leg, cfg.MaxKeepaliveRetry),
		regSeq:        0,
		registed:      0,
		keepaliveSeq:  0,
	}
	return reg, nil
}

func (r *Registar) Run(xlog *xlog.Logger, tr *transport.Transport) {
	//	raddr := r.tr.Conn.RemoteAddr().(*net.UDPAddr)
	regTimer := time.Tick(time.Duration(r.cfg.RegExpire) * time.Second)
	regRetry := time.Tick(time.Second * 5)
	keepaliveTimer := time.Tick(time.Duration(r.cfg.KeepaliveInterval) * time.Second)

	// send first  register
	laHost := tr.Conn.LocalAddr().(*net.UDPAddr).IP.String()
	laPort := tr.Conn.LocalAddr().(*net.UDPAddr).Port
	req := r.newRegMsg(false, laHost, laPort)
	tr.Send <- req

	for {
		select {
		case <-regTimer:
			req := r.newRegMsg(false, laHost, laPort)
			tr.Send <- req
		case <-regRetry:
			if atomic.LoadInt32(&r.registed) == 0 || atomic.LoadInt32(&r.keepaliveTimeoutCount) >= 3 {
				req := r.newRegMsg(false, laHost, laPort)
				tr.Send <- req
			}
		case <-keepaliveTimer:
			if atomic.LoadInt32(&r.registed) == 1 {
				atomic.AddInt32(&r.keepaliveTimeoutCount, 1)
				req := r.newKeepaliveMsg(laHost, laPort)
				r.keepaliveLegs[int(r.keepaliveSeq)%r.cfg.MaxKeepaliveRetry] = Leg{req.CallID, req.From.Param.Get("tag").Value}
				atomic.AddInt32(&r.keepaliveSeq, 1)
				atomic.AddInt32(&r.keepaliveTimeoutCount, 1)
				tr.Send <- req
			}
		case <-r.CloseChan:
			if atomic.CompareAndSwapInt32(&r.registed, 1, 0) {
				req := r.newRegMsg(true, laHost, laPort)
				tr.Send <- req
			}
		}
	}
}

func (r *Registar) newRegMsg(unReg bool, localHost string, localPort int) *sip.Msg {
	atomic.AddInt32(&r.regSeq, 1)
	expire := r.cfg.RegExpire
	if unReg {
		expire = 0
	}
	req := &sip.Msg{
		CSeq:       int(r.regSeq),
		CallID:     util.GenerateCallID(),
		Method:     sip.MethodRegister,
		CSeqMethod: sip.MethodRegister,
		UserAgent:  version.Version(),
		Request: &sip.URI{
			Scheme: "sip",
			User:   r.cfg.ServerID,
			Host:   r.cfg.Realm,
		},
		Via: &sip.Via{
			Version:  "2.0",
			Protocol: "SIP",
			Host:     localHost,
			Port:     uint16(localPort),
			Param:    &sip.Param{Name: "branch", Value: util.GenerateBranch()},
		},
		Contact: &sip.Addr{
			Uri: &sip.URI{
				User: r.cfg.GBID,
				Host: localHost,
				Port: uint16(localPort),
			},
		},
		From: &sip.Addr{
			Uri: &sip.URI{
				User: r.cfg.GBID,
				Host: r.cfg.Realm,
			},
			Param: &sip.Param{Name: "tag", Value: util.GenerateTag()},
		},
		To: &sip.Addr{
			Uri: &sip.URI{
				User: r.cfg.GBID,
				Host: r.cfg.Realm,
			},
		},
		Expires: expire,
	}
	if unReg {
		req.CallID = r.regLeg.callID
		req.From.Param = &sip.Param{Name: "tag", Value: r.regLeg.FromTag, Next: nil}
	}
	r.regLeg = &Leg{req.CallID, req.From.Param.Get("tag").Value}
	return req
}
func (r *Registar) HandleResponse(resp *sip.Msg) bool {
	if resp.CSeqMethod == sip.MethodRegister {
		r.handleRegResp(resp)
		return true
	}
	if resp.CSeqMethod == sip.MethodMessage && r.keepAliveLeg(resp) {
		atomic.StoreInt32(&r.keepaliveTimeoutCount, 0)
		return true
	}
	return false
}

func (r *Registar) keepAliveLeg(resp *sip.Msg) bool {
	for _, l := range r.keepaliveLegs {
		if l.callID == resp.CallID &&
			strings.EqualFold(l.FromTag, resp.From.Param.Get("tag").Value) {
			return true
		}
	}
	return false
}
func (r *Registar) handleRegResp(resp *sip.Msg) {
	if r.regLeg.callID == resp.CallID &&
		strings.EqualFold(r.regLeg.FromTag, resp.From.Param.Get("tag").Value) &&
		resp.Expires != 0 {
		atomic.StoreInt32(&r.registed, 1)
	}
}

type keepalive struct {
	XMLName  xml.Name `xml:"Notify"`
	Text     string   `xml:",chardata"`
	CmdType  string   `xml:"CmdType"`
	SN       string   `xml:"SN"`
	DeviceID string   `xml:"DeviceID"`
	Status   string   `xml:"Status"`
	Info     string   `xml:"Info"`
}

func (ke *keepalive) ContentType() string {
	return "Application/MANSCDP+xml"
}
func (ke *keepalive) Data() []byte {
	data, _ := xml.MarshalIndent(ke, "  ", "    ")
	return []byte(xml.Header + string(data))
}
func (r *Registar) newKeepaliveMsg(localHost string, localPort int) *sip.Msg {
	req := &sip.Msg{
		CSeq:       int(r.regSeq),
		CallID:     util.GenerateCallID(),
		Method:     sip.MethodMessage,
		CSeqMethod: sip.MethodMessage,
		UserAgent:  version.Version(),
		Request: &sip.URI{
			Scheme: "sip",
			User:   r.cfg.ServerID,
			Host:   r.cfg.Realm,
		},
		Via: &sip.Via{
			Version:  "2.0",
			Protocol: "SIP",
			Host:     localHost,
			Port:     uint16(localPort),

			Param: &sip.Param{Name: "branch", Value: util.GenerateBranch()},
		},
		Contact: &sip.Addr{
			Uri: &sip.URI{
				User: r.cfg.GBID,
				Host: localHost,
				Port: uint16(localPort),
			},
		},
		From: &sip.Addr{
			Uri: &sip.URI{
				User: r.cfg.GBID,
				Host: r.cfg.Realm,
			},
			Param: &sip.Param{Name: "tag", Value: util.GenerateTag()},
		},
		To: &sip.Addr{
			Uri: &sip.URI{
				User: r.cfg.ServerID,
				Host: r.cfg.Realm,
			},
		},
	}
	req.Payload = &keepalive{
		CmdType:  "Keepalive",
		SN:       strconv.Itoa(util.GenerateCSeq()),
		Status:   "OK",
		DeviceID: r.cfg.GBID,
	}
	return req
}
