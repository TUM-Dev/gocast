package dao

import (
	"github.com/TUM-Dev/gocast/model"
	"gorm.io/gorm"
	"time"
)

//go:generate mockgen -source=token.go -destination ../mock_dao/token.go

type TokenDao interface {
	AddToken(token model.Token) error

	GetToken(token string) (model.Token, error)
	GetAllTokens() ([]AllTokensDto, error)

	TokenUsed(token model.Token) error

	DeleteToken(id string) error
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

// GetAllTokens returns all tokens and the corresponding users name, email and lrz id
func (d tokenDao) GetAllTokens() ([]AllTokensDto, error) {
	var tokens []AllTokensDto
	err := DB.Raw("SELECT tokens.*, u.name as user_name, u.email as user_email, u.lrz_id as user_lrz_id FROM tokens JOIN users u ON u.id = tokens.user_id WHERE tokens.deleted_at IS null").Scan(&tokens).Error
	return tokens, err
}

// TokenUsed is called when a token is used. It sets the last_used field to the current time.
func (d tokenDao) TokenUsed(token model.Token) error {
	return DB.Model(&token).Update("last_use", time.Now()).Error
}

func (d tokenDao) DeleteToken(id string) error {
	return DB.Delete(&model.Token{}, id).Error
}

type AllTokensDto struct {
	model.Token
	UserName  string
	UserMail  string
	UserLrzID string
}
