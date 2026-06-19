package model

import (
	"testing"
)

func TestServiceTypeConstant(t *testing.T) {
	if ServiceType == 0 {
		t.Fatal("ServiceType must be a non-zero account type")
	}
	for _, other := range []int{AdminType, LecturerType, GenericType, StudentType} {
		if ServiceType == other {
			t.Fatalf("ServiceType (%d) collides with existing type", ServiceType)
		}
	}
}
