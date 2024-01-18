package tools

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/NaySoftware/go-fcm"
	"github.com/meilisearch/meilisearch-go"
	uuid "github.com/satori/go.uuid"
	"github.com/spf13/viper"
)

var Cfg Config
var Loc *time.Location

func LoadConfig() {
	initCache()
	var err error
	Loc, err = time.LoadLocation("Europe/Berlin")
	if err != nil {
		logger.Error("tools.config.LoadConfig: can't get time.location", "err", err)
	}
	initConfig()
}

const rsaKeySize = 2048

func initConfig() {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("/etc/TUM-Live/")
	viper.AddConfigPath("$HOME/.TUM-Live")
	viper.AddConfigPath(".")
	err := viper.ReadInConfig()
	if err != nil {
		if errors.Is(err, viper.ConfigFileNotFoundError{}) {
			logger.Warn("tools.config.LoadConfig: can't find config file", "err", err)
		} else {
			panic(fmt.Errorf("fatal error config file: %v", err))
		}
	}
	logger.Info("Using Config file " + viper.ConfigFileUsed())
	err = viper.Unmarshal(&Cfg)
	if err != nil {
		panic(fmt.Errorf("fatal error config file: %v", err))
	}

	// set defaults
	if Cfg.WorkerToken == "" {
		Cfg.WorkerToken = uuid.NewV4().String()
		viper.Set("workerToken", Cfg.WorkerToken)
		err = viper.WriteConfig()
		if err != nil {
			logger.Warn("Can't write out config ", "err", err)
		}
	}
	if Cfg.JWTKey == nil {
		logger.Info("Generating new JWT key")
		JWTKey, err := rsa.GenerateKey(rand.Reader, rsaKeySize)
		if err != nil {
			logger.Error("Can't generate JWT key", "err", err)
		}
		armoured := string(pem.EncodeToMemory(
			&pem.Block{
				Type:  "RSA PRIVATE KEY",
				Bytes: x509.MarshalPKCS1PrivateKey(JWTKey),
			},
		))
		viper.Set("jwtKey", armoured)
		err = viper.WriteConfig()
		if err != nil {
			logger.Warn("Can't write out config ", "err", err)
		}
		jwtKey = JWTKey
	} else {
		k, _ := pem.Decode([]byte(*Cfg.JWTKey))
		key, err := x509.ParsePKCS1PrivateKey(k.Bytes)
		if err != nil {
			logger.Error("Can't parse JWT key", "err", err)
			return
		}
		jwtKey = key
	}
	// allow overwriting database host with env var, mainly for testing with docker-compose
	if os.Getenv("DBHOST") != "" {
		Cfg.Db.Host = os.Getenv("DBHOST")
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
	Mail MailConfig `yaml:"mail"`
	Db   struct {
		User     string `yaml:"user"`
		Password string `yaml:"password"`
		Database string `yaml:"database"`
		Host     string `yaml:"host"`
		Port     uint   `yaml:"port"`
	} `yaml:"db"`
	Campus struct {
		Base        string   `yaml:"base"`
		Tokens      []string `yaml:"tokens"`
		CampusProxy *struct {
			Host   string `yaml:"host"`
			Scheme string `yaml:"scheme"`
		} `yaml:"campusProxy"`
		RelevantOrgs *[]string `yaml:"relevantOrgs"`
	} `yaml:"campus"`
	Ldap struct {
		URL         string `yaml:"url"`
		User        string `yaml:"user"`
		Password    string `yaml:"password"`
		BaseDn      string `yaml:"baseDn"`
		UserDn      string `yaml:"userDn"`
		UseForLogin bool   `yaml:"useForLogin"`
	} `yaml:"ldap"`
	Saml *struct {
		IdpMetadataURL string   `yaml:"idpMetadataURL"`
		Cert           string   `yaml:"cert"`
		Privkey        string   `yaml:"privkey"`
		EntityID       string   `yaml:"entityID"`
		RootURLs       []string `yaml:"rootURL"`
		IdpName        string   `yaml:"idpName"`
		IdpColor       string   `yaml:"idpColor"`
	} `yaml:"saml"`
	Paths struct {
		Static   string `yaml:"static"`
		Mass     string `yaml:"mass"`
		Branding string `yaml:"branding"`
	} `yaml:"paths"`
	Auths struct {
		SmpUser     string `yaml:"smpUser"` // todo, do we need this? Should this be in the lecture_halls table?
		SmpPassword string `yaml:"smpPassword"`
		PwrCrtlAuth string `yaml:"pwrCrtlAuth"`
		CamAuth     string `yaml:"camAuth"`
	} `yaml:"auths"`
	Alerts *struct {
		Matrix *struct {
			Username    string `yaml:"username"`
			Password    string `yaml:"password"`
			Homeserver  string `yaml:"homeserver"`
			LogRoomID   string `yaml:"logRoomID"`
			AlertRoomID string `yaml:"alertRoomId"`
		} `yaml:"matrix"`
	} `yaml:"alerts"`
	VoiceService *struct {
		Host string `yaml:"host"`
		Port string `yaml:"port"`
	}
	IngestBase  string  `yaml:"ingestBase"`
	WebUrl      string  `yaml:"webUrl"`
	WorkerToken string  `yaml:"workerToken"` // used for workers to join the worker pool
	JWTKey      *string `yaml:"jwtKey"`
	Meili       *struct {
		Host   string `yaml:"host"`
		ApiKey string `yaml:"apiKey"`
	} `yaml:"meili"`
	VodURLTemplate string `yaml:"vodURLTemplate"`
	CanonicalURL   string `yaml:"canonicalURL"`
	FCMServerKey   string `yaml:"fcmServerKey"`
}

type MailConfig struct {
	Sender            string `yaml:"sender"`
	Server            string `yaml:"server"`
	SMIMECert         string `yaml:"SMIMECert"`
	SMIMEKey          string `yaml:"SMIMEKey"`
	MaxMailsPerMinute int    `yaml:"maxMailsPerMinute"`
}

func (Config) GetJWTKey() *rsa.PrivateKey {
	return jwtKey
}

var ErrMeiliNotConfigured = errors.New("meilisearch is not configured")

func (c Config) GetMeiliClient() (*meilisearch.Client, error) {
	if c.Meili == nil {
		return nil, ErrMeiliNotConfigured
	}
	return meilisearch.NewClient(meilisearch.ClientConfig{Host: c.Meili.Host, APIKey: c.Meili.ApiKey}), nil
}

var ErrFCMNotConfigured = errors.New("Firebase Cloud Messaging is not configured")

func (c Config) GetFCMClient() (*fcm.FcmClient, error) {
	if c.FCMServerKey == "" {
		return nil, ErrFCMNotConfigured
	}
	return fcm.NewFcmClient(c.FCMServerKey), nil
}

var jwtKey *rsa.PrivateKey

// CookieSecure sets whether to use secure cookies or not, defaults to false in dev mode, true in production
var CookieSecure = false
