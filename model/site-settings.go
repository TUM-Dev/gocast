package model

import "gorm.io/gorm"

// SiteSetting stores key-value pairs for site-wide configuration
type SiteSetting struct {
	gorm.Model

	Key   string `gorm:"uniqueIndex;not null"`
	Value string `gorm:"not null"`
}

const (
	// SettingActiveTheme is the key for the currently active theme
	// Value should be one of the ThemeID constants, or empty string for default
	SettingActiveTheme = "active_theme"
)

// ThemeID represents a unique identifier for a theme
type ThemeID string

const (
	// ThemeDefault represents no special theme (default appearance)
	ThemeDefault ThemeID = ""
	// ThemeChristmas represents the Christmas/winter holiday theme
	ThemeChristmas ThemeID = "christmas"
)

// Theme represents a selectable site theme
type Theme struct {
	ID          ThemeID `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Icon        string  `json:"icon"` // FontAwesome icon class
}

// AvailableThemes returns all themes that can be selected
func AvailableThemes() []Theme {
	return []Theme{
		{
			ID:          ThemeDefault,
			Name:        "Default",
			Description: "Standard appearance without special effects",
			Icon:        "fa-circle-half-stroke",
		},
		{
			ID:          ThemeChristmas,
			Name:        "Christmas",
			Description: "Festive holiday theme with snowflakes",
			Icon:        "fa-snowflake",
		},
	}
}
