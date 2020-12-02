package reg

import (
	"crypto/md5"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"strings"
)

var (
	ErrNilTransport      = errors.New("Transport is nil")
	ErrBadChallenge      = errors.New("Challenge is bad")
	ErrAlgNotImplemented = errors.New("Alg not implemented")
)

type challenge struct {
	Realm     string
	Domain    string
	Nonce     string
	Opaque    string
	Stale     string
	Algorithm string
	Qop       string
}

func parseChallenge(input string) (*challenge, error) {
	const ws = " \n\r\t"
	const qs = `"`
	s := strings.Trim(input, ws)
	if !strings.HasPrefix(s, "Digest ") {
		return nil, ErrBadChallenge
	}
	s = strings.Trim(s[7:], ws)
	s = strings.TrimSpace(s)
	sl := strings.Split(s, ",")
	c := &challenge{
		Algorithm: "MD5",
	}
	var r []string
	for i := range sl {
		r = strings.SplitN(sl[i], "=", 2)
		switch r[0] {
		case "realm":
			c.Realm = strings.Trim(r[1], qs)
		case "domain":
			c.Domain = strings.Trim(r[1], qs)
		case "nonce":
			c.Nonce = strings.Trim(r[1], qs)
		case "opaque":
			c.Opaque = strings.Trim(r[1], qs)
		case "stale":
			c.Stale = strings.Trim(r[1], qs)
		case "algorithm":
			c.Algorithm = strings.Trim(r[1], qs)
		case "qop":
			//TODO(gavaletz) should be an array of strings?
			c.Qop = strings.Trim(r[1], qs)
		default:
			return nil, ErrBadChallenge
		}
	}
	return c, nil
}

type credentials struct {
	Username   string
	Realm      string
	Nonce      string
	DigestURI  string
	Algorithm  string
	Cnonce     string
	Opaque     string
	MessageQop string
	NonceCount int
	method     string
	password   string
}

func h(data string) string {
	hf := md5.New()
	io.WriteString(hf, data)
	return fmt.Sprintf("%x", hf.Sum(nil))
}

func kd(secret, data string) string {
	return h(fmt.Sprintf("%s:%s", secret, data))
}

func (c *credentials) ha1() string {
	return h(fmt.Sprintf("%s:%s:%s", c.Username, c.Realm, c.password))
}

func (c *credentials) ha2() string {
	return h(fmt.Sprintf("%s:%s", c.method, c.DigestURI))
}

func (c *credentials) resp(cnonce string) (string, error) {
	c.NonceCount++
	if c.MessageQop == "auth" {
		if cnonce != "" {
			c.Cnonce = cnonce
		} else {
			b := make([]byte, 8)
			io.ReadFull(rand.Reader, b)
			c.Cnonce = fmt.Sprintf("%x", b)[:16]
		}

		//		return h(fmt.Sprintf("%s:%s:%s", c.ha1(), c.Nonce, c.ha2())), nil
		return kd(c.ha1(), fmt.Sprintf("%s:%08x:%s:%s:%s",
			c.Nonce, c.NonceCount, c.Cnonce, c.MessageQop, c.ha2())), nil
	} else if c.MessageQop == "" {
		return kd(c.ha1(), fmt.Sprintf("%s:%s", c.Nonce, c.ha2())), nil
	}
	return "", ErrAlgNotImplemented
}

func (c *credentials) authorize() (string, error) {
	// Note that this is only implemented for MD5 and NOT MD5-sess.
	// MD5-sess is rarely supported and those that do are a big mess.
	if c.Algorithm != "MD5" {
		return "", ErrAlgNotImplemented
	}
	// Note that this is NOT implemented for "qop=auth-int".  Similarly the
	// auth-int server side implementations that do exist are a mess.
	if c.MessageQop != "auth" && c.MessageQop != "" {
		return "", ErrAlgNotImplemented
	}
	resp, err := c.resp("")
	if err != nil {
		return "", ErrAlgNotImplemented
	}
	sl := []string{fmt.Sprintf(`username="%s"`, c.Username)}
	sl = append(sl, fmt.Sprintf(`realm="%s"`, c.Realm))
	sl = append(sl, fmt.Sprintf(`nonce="%s"`, c.Nonce))
	sl = append(sl, fmt.Sprintf(`uri="%s"`, c.DigestURI))
	sl = append(sl, fmt.Sprintf(`response="%s"`, resp))
	if c.Algorithm != "" {
		sl = append(sl, fmt.Sprintf(`algorithm="%s"`, c.Algorithm))
	}
	if c.Opaque != "" {
		sl = append(sl, fmt.Sprintf(`opaque="%s"`, c.Opaque))
	}
	if c.MessageQop != "" {
		sl = append(sl, fmt.Sprintf("qop=%s", c.MessageQop))
		sl = append(sl, fmt.Sprintf("nc=%08x", c.NonceCount))
		sl = append(sl, fmt.Sprintf(`cnonce="%s"`, c.Cnonce))
	}
	return fmt.Sprintf("Digest %s", strings.Join(sl, ", ")), nil
}
