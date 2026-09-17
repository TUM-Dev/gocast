package apiv2

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strconv"

	uuid "github.com/satori/go.uuid"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	e "github.com/TUM-Dev/gocast/apiv2/errors"
	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
)

// Every RPC here is gated on PermManageUsers by its policy in services.go -- this is
// the only page that can issue or revoke a token at all, so unlike most of
// AdminService it does not sit behind server.administer.

// ListTokens lists every issued token's owner, scope and usage. It never returns a
// token's secret: that is shown once, in the response to createToken, and nowhere
// else afterwards -- not even to the account that created it.
func (a *API) ListTokens(ctx context.Context, req *emptypb.Empty) (*protobuf.ListTokensResponse, error) {
	caller, err := a.getCurrent(ctx)
	if err != nil {
		return nil, e.WithStatus(http.StatusUnauthorized, err)
	}

	// GetAllTokens scopes to the caller's own tokens without users.manage. Every
	// caller of this RPC holds it by policy, so this always returns every token --
	// kept anyway as the defense the dao already offers, in case that policy ever
	// loosens.
	tokens, err := a.dao.TokenDao.GetAllTokens(caller)
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	out := make([]*protobuf.Token, 0, len(tokens))
	for _, token := range tokens {
		out = append(out, tokenMessage(token))
	}

	return &protobuf.ListTokensResponse{Tokens: out, RtmpProxyUrl: tools.Cfg.RtmpProxyURL}, nil
}

// CreateToken issues a token and returns its secret. The secret is not stored
// anywhere the server can show it back: model.Token keeps only the value itself, so
// this response is the only time it is ever readable again.
func (a *API) CreateToken(ctx context.Context, req *protobuf.CreateTokenRequest) (*protobuf.TokenSecret, error) {
	caller, err := a.getCurrent(ctx)
	if err != nil {
		return nil, e.WithStatus(http.StatusUnauthorized, err)
	}

	scope := req.GetScope()
	if scope != model.TokenScopeAdmin && scope != model.TokenScopeLecturer {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("invalid scope"))
	}
	// Every caller of this RPC already holds users.manage, so this can never refuse
	// in practice; left in as the check v1 made, rather than assuming the policy
	// above is the only place that will ever gate this.
	if scope == model.TokenScopeAdmin && !caller.Can(model.PermManageUsers) {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("not an admin"))
	}

	expires := sql.NullTime{}
	if req.Expires != nil {
		expires = sql.NullTime{Valid: true, Time: req.Expires.AsTime()}
	}

	secret := uuid.NewV4().String()
	token := model.Token{
		UserID:  caller.ID,
		Token:   secret,
		Expires: expires,
		Scope:   scope,
	}
	if err := a.dao.TokenDao.AddToken(token); err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	return &protobuf.TokenSecret{Token: secret}, nil
}

// DeleteToken revokes a token; it stops authenticating immediately.
func (a *API) DeleteToken(ctx context.Context, req *protobuf.DeleteTokenRequest) (*emptypb.Empty, error) {
	caller, err := a.getCurrent(ctx)
	if err != nil {
		return nil, e.WithStatus(http.StatusUnauthorized, err)
	}

	id := strconv.FormatUint(uint64(req.GetId()), 10)
	token, err := a.dao.TokenDao.GetTokenByID(id)
	if err != nil {
		return nil, e.FromGorm(err, "can't find token")
	}

	// Every caller of this RPC already holds users.manage, so this ownership check
	// -- the one v1 made, to also let a token's own creator delete it -- can never
	// refuse in practice. Left in rather than assumed away, same as above.
	if token.UserID != caller.ID && !caller.Can(model.PermManageUsers) {
		return nil, e.WithStatus(http.StatusForbidden, errors.New("not allowed to delete token"))
	}

	if err := a.dao.TokenDao.DeleteToken(id); err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	return &emptypb.Empty{}, nil
}

func tokenMessage(token dao.AllTokensDto) *protobuf.Token {
	msg := &protobuf.Token{
		Id:        uint32(token.ID),
		UserName:  token.UserName,
		UserEmail: token.UserMail,
		UserLrzId: token.UserLrzID,
		Scope:     token.Scope,
	}
	if token.Expires.Valid {
		msg.Expires = timestamppb.New(token.Expires.Time)
	}
	if token.LastUse.Valid {
		msg.LastUse = timestamppb.New(token.LastUse.Time)
	}
	return msg
}
