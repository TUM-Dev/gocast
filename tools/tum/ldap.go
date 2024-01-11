package tum

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
	"github.com/getsentry/sentry-go"
	"github.com/go-ldap/ldap/v3"
	"time"
)

var ErrLdapBadAuth = errors.New("login failed")

type LdapResp struct {
	UserId    string
	LrzIdent  string
	FirstName string
	LastName  *string
}

// LoginWithTumCredentials returns student id if login and password match, err otherwise
func LoginWithTumCredentials(username string, password string) (*LdapResp, error) {
	// sanitize possibly malicious username
	username = ldap.EscapeFilter(username)
	defer sentry.Flush(time.Second * 2)
	l, err := ldap.DialURL(tools.Cfg.Ldap.URL)
	if err != nil {
		return nil, err
	}
	defer l.Close()

	// First bind with a read only user
	err = l.Bind(tools.Cfg.Ldap.User, tools.Cfg.Ldap.Password)
	if err != nil {
		return nil, err
	}

	// Search for the given username
	searchRequest := ldap.NewSearchRequest(
		"ou=users,ou=data,ou=prod,ou=iauth,dc=tum,dc=de",
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		fmt.Sprintf("(&(objectClass=organizationalPerson)(|(uid=%s)(imEmailAdressen=%s)))", username, username),
		[]string{"dn"},
		nil,
	)

	sr, err := l.Search(searchRequest)
	if err != nil {
		return nil, errors.New("couldn't query user")
	}

	if len(sr.Entries) != 1 {
		return nil, ErrLdapBadAuth
	}

	userdn := sr.Entries[0].DN
	// Bind as the user to verify their password
	err = l.Bind(userdn, password)
	if err != nil {
		return nil, ErrLdapBadAuth
	}
	res, err := l.Search(&ldap.SearchRequest{
		BaseDN:   userdn,
		Filter:   "(objectClass=Person)",
		Controls: nil,
	})
	if err != nil {
		return nil, errors.New("couldn't search ldap response")
	}

	if len(res.Entries) != 1 {
		return nil, errors.New("bad response from ldap server")
	}
	mNr := res.Entries[0].GetAttributeValue("imMatrikelNr")
	mwnID := res.Entries[0].GetAttributeValue("imMWNID")
	lrzID := res.Entries[0].GetAttributeValue("imLRZKennung")
	name := res.Entries[0].GetAttributeValue("imVorname")
	lastNameS := res.Entries[0].GetAttributeValue("sn")
	var lastName *string
	if lastNameS != "" {
		lastName = &lastNameS
	}
	uid := mNr
	if uid == "" {
		uid = mwnID
	}
	return &LdapResp{
		UserId:    uid,
		LrzIdent:  lrzID,
		FirstName: name,
		LastName:  lastName,
	}, nil

}

func FindUserWithEmail(email string) (*model.User, error) {
	username := ldap.EscapeFilter(email)
	defer sentry.Flush(time.Second * 2)
	l, err := ldap.DialURL(tools.Cfg.Ldap.URL)
	if err != nil {
		return nil, err
	}
	defer l.Close()

	// First bind with a read only user
	err = l.Bind(tools.Cfg.Ldap.User, tools.Cfg.Ldap.Password)
	if err != nil {
		return nil, err
	}

	// Search for the given username
	searchRequest := &ldap.SearchRequest{
		BaseDN: "ou=users,ou=data,ou=prod,ou=iauth,dc=tum,dc=de",
		Scope:  ldap.ScopeWholeSubtree,
		Filter: fmt.Sprintf("(imEmailAdressen=%s)", username),
	}

	sr, err := l.Search(searchRequest)
	if err != nil {
		return nil, errors.New("couldn't query user")
	}

	if len(sr.Entries) != 1 {
		return nil, ErrLdapBadAuth
	}

	mNr := sr.Entries[0].GetAttributeValue("imMatrikelNr")
	mwnID := sr.Entries[0].GetAttributeValue("imMWNID")
	lrzID := sr.Entries[0].GetAttributeValue("imLRZKennung")
	name := sr.Entries[0].GetAttributeValue("imVorname")
	if mNr == "" {
		mNr = mwnID
	}
	return &model.User{
		Name:                name,
		MatriculationNumber: mNr,
		LrzID:               lrzID,
		Email:               sql.NullString{String: email, Valid: true},
		Role:                model.LecturerType,
	}, nil
}
