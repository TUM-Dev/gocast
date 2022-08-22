package tools

import (
	"bytes"
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"html/template"
)

var Branding BrandingCfg

func init() {
	Branding.Init()
	renderManifestJSON()
}

func renderManifestJSON() {
	var manifest bytes.Buffer
	templ, _ := template.ParseFiles("tools/template/manifest.gotemplate")
	_ = templ.ExecuteTemplate(&manifest, "manifest.gotemplate", Branding.Manifest)
	fmt.Println(manifest.String())
}

type BrandingCfg struct {
	Manifest manifest `yaml:"manifest"`
}

type manifest struct {
	Name        string `yaml:"name"`
	ShortName   string `yaml:"shortname"`
	Icons       []icon `yaml:"icons"`
	Description string `yaml:"description"`
	BgColor     string `yaml:"bgcolor"`
}

func (m manifest) MaxIconIndex() int {
	return len(m.Icons) - 1
}

type icon struct {
	Src       string `yaml:"src"`
	ImageType string `yaml:"imagetype"`
	Sizes     string `yaml:"sizes"`
}

func (b *BrandingCfg) Init() {
	v := viper.New()
	v.SetConfigName("branding")
	v.SetConfigType("yaml")
	v.AddConfigPath("/etc/TUM-Live/")
	v.AddConfigPath("$HOME/.TUM-Live")
	v.AddConfigPath(".")

	err := v.ReadInConfig()
	if err != nil {
		if err == err.(viper.ConfigFileNotFoundError) {
			log.WithError(err).Warn("tools.config.init: can't find config file")
		} else {
			panic(fmt.Errorf("fatal error config file: %v", err))
		}
	}

	log.Info("Using BrandingCfg file ", v.ConfigFileUsed())
	err = v.Unmarshal(&b)
	if err != nil {
		panic(fmt.Errorf("fatal error config file: %v", err))
	}
}
