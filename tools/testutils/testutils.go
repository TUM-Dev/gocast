package testutils

import (
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/tools"
	"io"
)

type TestCases map[string]TestCase

type TestCase struct {
	Method         string
	Url            string
	DaoWrapper     dao.DaoWrapper
	TumLiveContext *tools.TUMLiveContext
	Body           io.Reader
	ExpectedCode   int
}

func First(a interface{}, b interface{}) interface{} {
	return a
}

func Second(a interface{}, b interface{}) interface{} {
	return b
}
