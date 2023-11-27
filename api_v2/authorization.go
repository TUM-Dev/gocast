package api_v2

import (
	"context"
	"errors"
	e "github.com/TUM-Dev/gocast/api_v2/errors"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
	"github.com/golang-jwt/jwt/v4"
	"google.golang.org/grpc/metadata"
	"gorm.io/gorm"
	"net/http"
	"strings"
)

func (a *API) getCurrent(ctx context.Context) (*model.User, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, errors.New("no metadata")
	}

	jwtStr, err := a.extractJWTFromMetadata(md)
	if err != nil {
		return nil, err
	}

	claims, err := a.parseJWT(jwtStr)
	if err != nil {
		return nil, err
	}

	return a.getUserFromClaims(claims)
}

func (a *API) extractJWTFromMetadata(md metadata.MD) (string, error) {
	cookies, ok := md["grpcgateway-cookie"]
	if !ok || len(cookies) < 1 {
		return "", errors.New("missing cookie header")
	}

	return extractTokenFromCookie(cookies[0])
}

func extractTokenFromCookie(cookieHeader string) (string, error) {
	cookies := strings.Split(cookieHeader, ";")
	for _, cookie := range cookies {
		cookie = strings.TrimSpace(cookie)
		if strings.HasPrefix(cookie, "jwt=") {
			return strings.TrimPrefix(cookie, "jwt="), nil
		}
	}

	return "", errors.New("jwt cookie not found")
}

func (a *API) parseJWT(jwtStr string) (*tools.JWTClaims, error) {
	token, err := jwt.ParseWithClaims(jwtStr, &tools.JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		return tools.Cfg.GetJWTKey().Public(), nil
	})

	if err != nil {
		a.log.Info("JWT parsing error: ", err)
		return nil, err
	}

	if !token.Valid {
		a.log.Info("JWT token is not valid")
		return nil, errors.New("JWT token is not valid")
	}

	claims, ok := token.Claims.(*tools.JWTClaims)
	if !ok {
		return nil, errors.New("error extracting claims from token")
	}

	return claims, nil
}

func (a *API) getUserFromClaims(claims *tools.JWTClaims) (*model.User, error) {
	var u model.User
	err := a.db.Where("id = ?", claims.UserID).First(&u).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	
	return &u, nil
}
