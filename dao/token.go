package dao

import (
	"TUM-Live/model"
	"time"
)

// GetToken returns the first token for the given string that is not expired.
func GetToken(token string) (model.Token, error) {
	var t model.Token
	err := DB.Model(&t).Where("token = ? AND expires > NOW()", token).First(&t).Error
	return t, err
}

// AddToken adds a new token to the database
func AddToken(token model.Token) error {
	return DB.Create(&token).Error
}

func DeleteToken(id string) error {
	return DB.Delete(&model.Token{}, id).Error
}

// TokenUsed is called when a token is used. It sets the last_used field to the current time.
func TokenUsed(token model.Token) error {
	return DB.Model(&token).Update("last_use", time.Now()).Error
}

// GetAllTokens returns all tokens and the corresponding users name, email and lrz id
func GetAllTokens() ([]AllTokensDto, error) {
	var tokens []AllTokensDto
	err := DB.Raw("SELECT tokens.*, u.name as user_name, u.email as user_email, u.lrz_id as user_lrz_id FROM tokens JOIN users u ON u.id = tokens.user_id WHERE tokens.deleted_at IS null").Scan(&tokens).Error
	return tokens, err
}

type AllTokensDto struct {
	model.Token
	UserName  string
	UserMail  string
	UserLrzID string
}
