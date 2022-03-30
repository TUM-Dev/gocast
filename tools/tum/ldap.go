package tum

import (
	"TUM-Live/tools"
	"errors"
	"fmt"
	"github.com/getsentry/sentry-go"
	"github.com/go-ldap/ldap/v3"
	"time"
)

var ErrLdapBadAuth = errors.New("login failed")

//LoginWithTumCredentials returns student id if login and password match, err otherwise
func LoginWithTumCredentials(username string, password string) (userId string, lrzIdent string, firstName string, err error) {
	// sanitize possibly malicious username
	username = ldap.EscapeFilter(username)
	defer sentry.Flush(time.Second * 2)
	l, err := ldap.DialURL(tools.Cfg.Ldap.URL)
	if err != nil {
		return "", "", "", err
	}
	defer l.Close()

	// First bind with a read only user
	err = l.Bind(tools.Cfg.Ldap.User, tools.Cfg.Ldap.Password)
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
		return "", "", "", ErrLdapBadAuth
	}

	userdn := sr.Entries[0].DN
	// Bind as the user to verify their password
	err = l.Bind(userdn, password)
	if err != nil {
		return "", "", "", ErrLdapBadAuth
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

func FindUserWithTumId(tumId string) error {
	username := ldap.EscapeFilter(tumId)
	defer sentry.Flush(time.Second * 2)
	l, err := ldap.DialURL(tools.Cfg.Ldap.URL)
	if err != nil {
		return err
	}
	defer l.Close()

	// First bind with a read only user
	err = l.Bind(tools.Cfg.Ldap.User, tools.Cfg.Ldap.Password)
	if err != nil {
		return err
	}

	// Search for the given username
	searchRequest := &ldap.SearchRequest{
		BaseDN: "ou=users,ou=data,ou=prod,ou=iauth,dc=tum,dc=de",
		Scope:  ldap.ScopeWholeSubtree,
		Filter: fmt.Sprintf("(imEmailAdressen=%s)", username),
	}

	sr, err := l.Search(searchRequest)
	if err != nil {
		return errors.New("couldn't query user")
	}

	if len(sr.Entries) != 1 {
		return ErrLdapBadAuth
	}
	printResult(sr.Entries)
	return fmt.Errorf("LDAP: reached unexpected codepoint. User: %v", username)
}

func printResult(entries []*ldap.Entry) {
	for _, entry := range entries {
		fmt.Println("DN:", entry.DN)
		for _, attr := range entry.Attributes {
			for i := 0; i < len(attr.Values); i++ {
				fmt.Printf("%s: %s\n", attr.Name, attr.Values[i])
			}
		}
		fmt.Println()
	}
}
