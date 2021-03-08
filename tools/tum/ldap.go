package tum

import (
	"TUM-Live/tools"
	"errors"
	"fmt"
	"github.com/go-ldap/ldap/v3"
	"log"
)

/**
 * returns student id if login and password match, err otherwise
 */
func LoginWithTumCredentials(lrzId string, password string) (userId string, err error) {
	l, err := ldap.DialURL(tools.Cfg.LdapUrl)
	if err != nil {
		log.Fatal(err)
	}
	defer l.Close()

	// First bind with a read only user
	err = l.Bind(tools.Cfg.LdapUser, tools.Cfg.LdapPassword)
	if err != nil {
		log.Fatal(err)
	}

	// Search for the given username
	searchRequest := ldap.NewSearchRequest(
		"ou=users,ou=data,ou=prod,ou=iauth,dc=tum,dc=de",
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		fmt.Sprintf("(&(objectClass=organizationalPerson)(uid=%s))", lrzId),
		[]string{"dn"},
		nil,
	)

	sr, err := l.Search(searchRequest)
	if err != nil {
		return "", errors.New("couldn't query user")
	}

	if len(sr.Entries) != 1 {
		log.Printf("User does not exist or too many entries returned: %v\n", len(sr.Entries))
		return "", errors.New("couldn't find single user")
	}

	userdn := sr.Entries[0].DN
	// Bind as the user to verify their password
	err = l.Bind(userdn, password)
	if err != nil {
		log.Printf("login failed\n")
		return "", errors.New("couldn't login with tum credentials")
	}
	res, err := l.Search(&ldap.SearchRequest{
		BaseDN:   fmt.Sprintf(tools.Cfg.LdapUserDN, lrzId),
		Filter:   "(objectClass=Person)",
		Controls: nil,
	})
	if err != nil {
		log.Printf("login failed\n")
		return "", errors.New("couldn't login with tum credentials")
	} else {
		if len(res.Entries)!=1 {
			log.Println("bad response from ldap server")
			return "", errors.New("bad response from ldap server")
		}
		mNr := res.Entries[0].GetAttributeValue("imMatrikelNr")
		if mNr != "" {
			return mNr, nil
		}
	}
	return "", errors.New("something went wrong")
}
