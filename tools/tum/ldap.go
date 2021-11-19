package tum

import (
	"TUM-Live/tools"
	"errors"
	"fmt"
	"github.com/getsentry/sentry-go"
	"github.com/go-ldap/ldap/v3"
	"time"
)

var LdapErrBadAuth = errors.New("login failed")

//LoginWithTumCredentials returns student id if login and password match, err otherwise
func LoginWithTumCredentials(username string, password string) (userId string, lrzIdent string, firstName string, err error) {
	// sanitize possibly malicious username
	username = ldap.EscapeFilter(username)
	defer sentry.Flush(time.Second * 2)
	l, err := ldap.DialURL(tools.Cfg.LdapUrl)
	if err != nil {
		return "", "", "", err
	}
	defer l.Close()

	// First bind with a read only user
	err = l.Bind(tools.Cfg.LdapUser, tools.Cfg.LdapPassword)
	if err != nil {
		return "", "", "", err
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
		return "", "", "", errors.New("couldn't query user")
	}

	if len(sr.Entries) != 1 {
		return "", "", "", LdapErrBadAuth
	}

	userdn := sr.Entries[0].DN
	// Bind as the user to verify their password
	err = l.Bind(userdn, password)
	if err != nil {
		return "", "", "", LdapErrBadAuth
	}
	res, err := l.Search(&ldap.SearchRequest{
		BaseDN:   userdn,
		Filter:   "(objectClass=Person)",
		Controls: nil,
	})
	if err != nil {
		return "", "", "", errors.New("couldn't search ldap response")
	} else {
		if len(res.Entries) != 1 {
			return "", "", "", errors.New("bad response from ldap server")
		}
		mNr := res.Entries[0].GetAttributeValue("imMatrikelNr")
		mwnID := res.Entries[0].GetAttributeValue("imMWNID")
		lrzID := res.Entries[0].GetAttributeValue("imLRZKennung")
		name := res.Entries[0].GetAttributeValue("imVorname")
		if mNr != "" {
			return mNr, lrzID, name, nil
		}
		if mwnID != "" {
			return mwnID, lrzID, name, nil
		}
	}
	return "", "", "", fmt.Errorf("LDAP: reached unexpected codepoint. User: %v", username)
}
