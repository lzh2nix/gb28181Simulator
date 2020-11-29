package config

import (
	"encoding/json"
	"io/ioutil"
	"os"
)

type Config struct {
	LocalSipPort      int          `json:"localSipPort"`
	ServerID          string       `json:"serverID"`
	Realm             string       `json:"realm"`
	ServerAddr        string       `json:"serverAddr"`
	UserName          string       `json:"userName"`
	Password          string       `json:"password"`
	RegExpire         int          `json:"regExpire"`
	KeepaliveInterval int          `json:"keepaliveInterval"`
	MaxKeepaliveRetry int          `json:"maxKeepaliveRetry"`
	Transport         string       `json:"transport"`
	GBID              string       `json:"gbID"`
	Devices           []DeviceInfo `json:"devices"`
}

type DeviceInfo struct {
	Text         string `xml:",chardata"`
	DeviceID     string `xml:"DeviceID" json:"deviceID"`
	Name         string `xml:"Name" json:"name""`
	Manufacturer string `xml:"Manufacturer" json:"manufacturer"`
	Model        string `xml:"Model" json:"model"`
	Owner        string `xml:"Owner" json:"owner"`
	CivilCode    string `xml:"CivilCode" json:"civilCode"`
	Address      string `xml:"Address" json:"address"`
	Parental     string `xml:"Parental" json:"parental"`
	SafetyWay    string `xml:"SafetyWay" json:"safeWay"`
	RegisterWay  string `xml:"RegisterWay" json:"registerWay"`
	Secrecy      string `xml:"Secrecy" json:"secrecy"`
	Status       string `xml:"Status" json:"status"`
}

func ParseJsonConfig(f *string) (*Config, error) {
	jsonFile, err := os.Open(*f)
	if err != nil {
		return nil, err
	}
	// defer the closing of our jsonFile so that we can parse it later on
	defer jsonFile.Close()

	b, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		return nil, err
	}
	var cfg Config
	err = json.Unmarshal(b, &cfg)
	return &cfg, err
}
