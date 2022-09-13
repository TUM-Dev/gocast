package tools

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

var BrandingCfg Branding

type Branding struct {
	Title       string `yaml:"title"`
	Description string `yaml:"description"`
}

func getDefault() Branding {
	return Branding{
		Title: "TUM-Live",
		Description: "TUM-Live, the livestreaming and VoD service of the " +
			"Rechnerbetriebsgruppe at the department of informatics and " +
			"mathematics at the Technical University of Munich",
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
