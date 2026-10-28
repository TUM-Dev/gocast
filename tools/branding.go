package tools

import (
	"errors"
	"fmt"
)

var BrandingCfg Branding

type Branding struct {
	Title       string `yaml:"title"`
	Description string `yaml:"description"`
}

// getDefaultBranding returns the struct branding with default values
func getDefaultBranding() Branding {
	return Branding{
		Title: "TUM-Live",
		Description: "TUM-Live, the livestreaming and VoD service of the " +
			"Rechnerbetriebsgruppe at the department of informatics and " +
			"mathematics at the Technical University of Munich",
	}
}

// InitBranding initializes the global branding configuration variable `BrandingCfg`. If the config file doesn't exist
// it will be set to the result of `getDefaultBranding()`.
func InitBranding() {
	branding := getDefaultBranding()

	path, err := loadConfigFile("branding", &branding)
	switch {
	case errors.Is(err, errConfigNotFound):
	case err != nil:
		panic(fmt.Errorf("fatal error branding file: %v", err))
	default:
		logger.Info("Using " + path)
	}

	BrandingCfg = branding
}
