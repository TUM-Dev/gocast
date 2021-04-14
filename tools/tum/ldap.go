package tum

import (
	"TUM-Live/tools"
	"errors"
	"fmt"
	"github.com/getsentry/sentry-go"
	"github.com/go-ldap/ldap/v3"
	"log"
	"time"
)

/**
 * returns student id if login and password match, err otherwise
 */
func LoginWithTumCredentials(username string, password string) (userId string, firstName string, err error) {
	defer sentry.Flush(time.Second * 2)
	l, err := ldap.DialURL(tools.Cfg.LdapUrl)
	if err != nil {
		sentry.CaptureException(err)
		log.Fatal(err)
	}
	defer l.Close()

	// First bind with a read only user
	err = l.Bind(tools.Cfg.LdapUser, tools.Cfg.LdapPassword)
	if err != nil {
		sentry.CaptureException(err)
		log.Fatal(err)
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
		sentry.CaptureException(err)
		return "", "", errors.New("couldn't query user")
	}

	if len(sr.Entries) != 1 {
		log.Printf("User does not exist or too many entries returned: %v\n", len(sr.Entries))
		return "", "", errors.New("couldn't find single user")
	}

	userdn := sr.Entries[0].DN
	// Bind as the user to verify their password
	err = l.Bind(userdn, password)
	if err != nil {
		log.Printf("%v\n", err)
		return "", "", errors.New("couldn't login with tum credentials")
	}
	res, err := l.Search(&ldap.SearchRequest{
		BaseDN:   userdn,
		Filter:   "(objectClass=Person)",
		Controls: nil,
	})
	if err != nil {
		log.Printf("%v\n", err)
		return "", "", errors.New("couldn't login with tum credentials")
	} else {
		if len(res.Entries) != 1 {
			sentry.CaptureMessage(fmt.Sprintf("bad response from ldap server. User: %v", username))
			return "", "", errors.New("bad response from ldap server")
		}
		mNr := res.Entries[0].GetAttributeValue("imMatrikelNr")
		mwnID := res.Entries[0].GetAttributeValue("imMWNID")
		name := res.Entries[0].GetAttributeValue("imVorname")
		if mNr != "" {
			return mNr, name, nil
		}
		if mwnID != "" {
			log.Println("Falling back to mwn id.")
			return mwnID, name, nil
		}
	}
	sentry.CaptureMessage(fmt.Sprintf("LDAP: reached unexpected codepoint. User: %v", username))
	return "", "", errors.New("something went wrong")
}
