package web

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	loginInternal = "internal"
	loginSAML     = "saml"
	loginLDAP     = "ldap"
)

var loginAttempts = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "gocast_login_attempts_total",
	Help: "Login attempts by method (internal, saml, ldap) and result (success, failure)",
}, []string{"method", "result"})

func countLogin(method string, ok bool) {
	result := "failure"
	if ok {
		result = "success"
	}
	loginAttempts.WithLabelValues(method, result).Inc()
}
