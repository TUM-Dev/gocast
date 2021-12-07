package tools

import (
	log "github.com/sirupsen/logrus"
	"os"
	"strings"
	"time"
)

var Cfg Config
var Loc *time.Location

func init() {
	var err error
	Loc, err = time.LoadLocation("Europe/Berlin")
	if err != nil {
		log.WithError(err).Error("tools.config.init: can't get time.location")
	}
	Cfg = Config{
		MailUser:             os.Getenv("MAIL_USER"),
		MailServer:           os.Getenv("MAIL_SERVER"),
		DatabaseUser:         os.Getenv("MYSQL_USER"),
		DatabasePassword:     os.Getenv("MYSQL_PASSWORD"),
		DatabaseName:         os.Getenv("MYSQL_DATABASE"),
		VersionTag:           os.Getenv("VERSION_TAG"),
		LrzServerIngest:      os.Getenv("LRZ_SERVER_INGEST"),
		LrzServerHls:         os.Getenv("LRZ_SERVER_HLS"),
		LrzPassword:          os.Getenv("LRZ_PASSWORD"),
		CampusBase:           os.Getenv("CAMPUS_API_BASE"),
		CampusToken:          strings.Split(os.Getenv("CAMPUS_API_TOKEN"), ";"),
		CookieStoreSecret:    os.Getenv("COOKIE_STORE_SECRET"),
		LdapUrl:              os.Getenv("LDAP_URL"),
		LdapUser:             os.Getenv("LDAP_USER"),
		LdapPassword:         os.Getenv("LDAP_PASSWORD"),
		LdapBaseDN:           os.Getenv("LDAP_BASE_DN"),
		LdapUserDN:           os.Getenv("LDAP_USER_DN"),
		IngestBase:           os.Getenv("IngestBase"),
		CameraAuthentication: os.Getenv("CAMERA_AUTH"),
		StaticPath:           os.Getenv("STATIC_PATH"),
		MassStorage:          os.Getenv("MASS_STORAGE"),
		SMPUser:              os.Getenv("SMP_USER"),
		SMPPassword:          os.Getenv("SMP_PASSWORD"),
		PWRCTRLAuth:          os.Getenv("PWRCTRL_AUTH"),
		SMIMECert:            os.Getenv("SMIMECRT"),
		SMIMEKey:             os.Getenv("SMIMEKEY"),
		LRZUploadURL:         os.Getenv("LRZ_UPLOAD_URL"),
		LrzUser:              os.Getenv("LRZ_USER"),
		LRZPhone:             os.Getenv("LRZ_PHONE"),
		LRZMail:              os.Getenv("LRZ_MAIL"),
		LRZSubDir:            os.Getenv("LRZ_SUBDIR"),
	}
}

type Config struct {
	MailUser             string
	MailServer           string
	DatabaseUser         string
	DatabasePassword     string
	DatabaseName         string
	VersionTag           string
	LrzServerIngest      string
	LrzServerHls         string
	LrzUser              string
	LrzPassword          string
	CampusBase           string
	CampusToken          []string
	CookieStoreSecret    string
	LdapUrl              string
	LdapUser             string
	LdapPassword         string
	LdapBaseDN           string
	LdapUserDN           string
	IngestBase           string
	CameraAuthentication string
	StaticPath           string
	MassStorage          string
	SMPUser              string
	SMPPassword          string
	PWRCTRLAuth          string
	SMIMECert            string
	SMIMEKey             string
	LRZUploadURL         string
	LRZPhone             string
	LRZMail              string
	LRZSubDir            string
}
