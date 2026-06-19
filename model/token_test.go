package model

import (
	"testing"
)

func TestTokenScopeServiceConstant(t *testing.T) {
	if TokenScopeService != "service" {
		t.Fatalf("TokenScopeService must equal \"service\", got %q", TokenScopeService)
	}
}
