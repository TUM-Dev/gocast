package tools

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"time"
)

var Cfg Config
var Loc *time.Location

func init() {
	initCache()
	var err error
	Loc, err = time.LoadLocation("Europe/Berlin")
	if err != nil {
		log.WithError(err).Error("tools.config.init: can't get time.location")
	}
	initConfig()
}

func initConfig() {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("/etc/TUM-Live/")
	viper.AddConfigPath("$HOME/.TUM-Live")
	viper.AddConfigPath(".")
	err := viper.ReadInConfig()
	if err != nil {
		if err == err.(viper.ConfigFileNotFoundError) {
			log.WithError(err).Warn("tools.config.init: can't find config file")
		} else {
			panic(fmt.Errorf("fatal error config file: %v", err))
		}
	}
	err = viper.Unmarshal(&Cfg)
	if err != nil {
		panic(fmt.Errorf("fatal error config file: %v", err))
	}
}

type Config struct {
	Lrz struct {
		Name      string `yaml:"name"`
		Email     string `yaml:"email"`
		Phone     string `yaml:"phone"`
		UploadURL string `yaml:"uploadUrl"`
		SubDir    string `yaml:"subDir"`
	} `yaml:"lrz"`
	Mail struct {
		Sender    string `yaml:"sender"`
		Server    string `yaml:"server"`
		SMIMECert string `yaml:"SMIMECert"`
		SMIMEKey  string `yaml:"SMIMEKey"`
	} `yaml:"mail"`
	Db struct {
		User     string `yaml:"user"`
		Password string `yaml:"password"`
		Database string `yaml:"database"`
	} `yaml:"db"`
	Campus struct {
		Base   string   `yaml:"base"`
		Tokens []string `yaml:"tokens"`
	} `yaml:"campus"`
	Matrix struct {
		Username   string `yaml:"username"`
		Password   string `yaml:"password"`
		RoomID     string `yaml:"roomID"`
		Homeserver string `yaml:"homeserver"`
	} `yaml:"matrix"`
	Ldap struct {
		URL      string `yaml:"url"`
		User     string `yaml:"user"`
		Password string `yaml:"password"`
		BaseDn   string `yaml:"baseDn"`
		UserDn   string `yaml:"userDn"`
	} `yaml:"ldap"`
	Paths struct {
		Static string `yaml:"static"`
		Mass   string `yaml:"mass"`
	} `yaml:"paths"`
	Auths struct {
		SmpUser     string `yaml:"smpUser"`
		SmpPassword string `yaml:"smpPassword"`
		PwrCrtlAuth string `yaml:"pwrCrtlAuth"`
		CamAuth     string `yaml:"camAuth"`
	} `yaml:"auths"`
	IngestBase        string `yaml:"ingestBase"`
	WebUrl            string `yaml:"webUrl"`
	CookieStoreSecret string `yaml:"cookieStoreSecret"`
}
