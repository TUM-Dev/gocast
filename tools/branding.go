package tools

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

var BrandingCfg Branding

type Branding struct {
	Title string `yaml:"title"`
}

func getDefault() Branding {
	return Branding{
		Title: "TUM-Live",
	}
}

func init() {
	v := viper.New()
	v.SetConfigName("branding")
	v.SetConfigType("yaml")
	v.AddConfigPath("/etc/TUM-Live/")
	v.AddConfigPath("$HOME/.TUM-Live")
	v.AddConfigPath(".")

	branding := getDefault()

	err := v.ReadInConfig()
	if err == nil {
		err = v.Unmarshal(&branding)
		log.Info("Using branding.yaml.")
		if err != nil {
			panic(fmt.Errorf("fatal error branding file: %v", err))
		}
	}

	BrandingCfg = branding
}
