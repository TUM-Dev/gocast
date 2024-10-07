package migrations

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

func Migrate2024090240() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "202310010",
		Migrate: func(tx *gorm.DB) error {
			// Update user roles
			if err := tx.Exec("UPDATE users SET role = role + 1 WHERE role = 4").Error; err != nil {
				return err
			}
			if err := tx.Exec("UPDATE users SET role = role + 1 WHERE role = 3").Error; err != nil {
				return err
			}
			if err := tx.Exec("UPDATE users SET role = role + 1 WHERE role = 2").Error; err != nil {
				return err
			}

			// Create the CIT organization
			if err := tx.Exec("INSERT INTO organizations (name, created_at, updated_at) VALUES ('TEST', now(), now())").Error; err != nil {
				return err
			}

			var citID int
			if err := tx.Raw("SELECT id FROM organizations WHERE name = 'CIT'").Scan(&citID).Error; err != nil {
				return err
			}

			// Update all existing courses, ingest_servers and workers to be assigned to the CIT organization
			if err := tx.Exec("UPDATE courses SET organization_id = ?", citID).Error; err != nil {
				return err
			}
			if err := tx.Exec("UPDATE ingest_servers SET organization_id = ?, shared = TRUE", citID).Error; err != nil {
				return err
			}
			if err := tx.Exec("UPDATE workers SET organization_id = ?, ingest = FALSE, shared = TRUE, address = host+'.cit.tum.de'", citID).Error; err != nil {
				return err
			}

			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			if err := tx.Exec("UPDATE users SET role = role + 1 WHERE role IN (1, 2, 3)").Error; err != nil {
				return err
			}

			var citID int
			if err := tx.Raw("SELECT id FROM organizations WHERE name = 'CIT'").Scan(&citID).Error; err != nil {
				return err
			}

			if err := tx.Exec("UPDATE courses SET organization_id = NULL WHERE organization_id = ?", citID).Error; err != nil {
				return err
			}

			if err := tx.Exec("DELETE FROM organizations WHERE id = ?", citID).Error; err != nil {
				return err
			}

			return nil
		},
	}
}
