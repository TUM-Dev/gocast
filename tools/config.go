package tools

var Cfg Config

type Config struct {
	MailUser         string
	MailPassword     string
	MailServer       string
	DatabaseUser     string
	DatabasePassword string
	DatabaseName     string
	VersionTag       string
	LrzServer        string
	LrzUser          string
	LrzPassword      string
}
