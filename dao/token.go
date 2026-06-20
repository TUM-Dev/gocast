package dao

import (
	"time"

	"gorm.io/gorm"

	"github.com/TUM-Dev/gocast/model"
)

//go:generate go tool mockgen -source=token.go -destination ../mock_dao/token.go

type TokenDao interface {
	AddToken(token model.Token) error

	GetToken(token string) (model.Token, error)
	GetTokenByID(id string) (model.Token, error)
	GetAllTokens(user *model.User) ([]AllTokensDto, error)

	TokenUsed(token model.Token) error

	DeleteToken(id string) error
	// DeleteServiceTokensForUser removes all service-scoped tokens for the given
	// user. This is called during provisioning so that re-running the provisioning
	// tool rotates credentials atomically (revoke old → insert new).
	DeleteServiceTokensForUser(userID uint) error
	// RotateServiceToken atomically removes all existing service-scoped tokens for
	// userID and inserts newToken in a single database transaction. If either
	// operation fails the transaction is rolled back so no tokens are lost or
	// duplicated.
	RotateServiceToken(userID uint, newToken model.Token) error
}

type tokenDao struct {
	db *gorm.DB
}

func NewTokenDao() TokenDao {
	return tokenDao{db: DB}
}

// AddToken adds a new token to the database
func (d tokenDao) AddToken(token model.Token) error {
	return DB.Create(&token).Error
}

// GetToken returns the first token for the given string that is not expired.
func (d tokenDao) GetToken(token string) (model.Token, error) {
	var t model.Token
	err := DB.Model(&t).Where("token = ? AND (expires IS null OR expires > NOW())", token).First(&t).Error
	return t, err
}

func (d tokenDao) GetTokenByID(id string) (model.Token, error) {
	var t model.Token
	err := DB.Model(&t).Where("id = ?", id).First(&t).Error
	return t, err
}

// GetAllTokens returns all tokens and the corresponding users name, email and lrz id
func (d tokenDao) GetAllTokens(user *model.User) ([]AllTokensDto, error) {
	var tokens []AllTokensDto

	query := DB.Table("tokens").
		Select("tokens.*, u.name as user_name, u.email as user_email, u.lrz_id as user_lrz_id").
		Joins("JOIN users u ON u.id = tokens.user_id").
		Where("tokens.deleted_at IS NULL")

	if user.Role != model.AdminType {
		query = query.Where("tokens.user_id = ?", user.ID)
	}

	err := query.Scan(&tokens).Error
	if err != nil {
		return nil, err
	}

	return tokens, nil
}

// TokenUsed is called when a token is used. It sets the last_used field to the current time.
func (d tokenDao) TokenUsed(token model.Token) error {
	return DB.Model(&token).Update("last_use", time.Now()).Error
}

func (d tokenDao) DeleteToken(id string) error {
	return DB.Delete(&model.Token{}, id).Error
}

// DeleteServiceTokensForUser deletes all service-scoped tokens for the given
// user. It performs a hard delete (Unscoped) so that the rows are removed
// rather than soft-deleted, preventing accumulation of stale credential rows.
func (d tokenDao) DeleteServiceTokensForUser(userID uint) error {
	return DB.Unscoped().
		Where("user_id = ? AND scope = ?", userID, model.TokenScopeService).
		Delete(&model.Token{}).Error
}

// RotateServiceToken atomically revokes all existing service-scoped tokens for
// userID and inserts newToken in a single database transaction. If the insert
// fails the delete is rolled back, so concurrent runs can never leave the user
// with zero valid tokens after a partial failure.
func (d tokenDao) RotateServiceToken(userID uint, newToken model.Token) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().
			Where("user_id = ? AND scope = ?", userID, model.TokenScopeService).
			Delete(&model.Token{}).Error; err != nil {
			return err
		}
		return tx.Create(&newToken).Error
	})
}

type AllTokensDto struct {
	model.Token
	UserName  string
	UserMail  string
	UserLrzID string
}
