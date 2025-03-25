package dao

import "github.com/dgraph-io/ristretto/v2"

var Cache *ristretto.Cache[string, any]
