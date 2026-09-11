package xray_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/iqhive/runtime.link/api/xray"
)

func TestErrors(t *testing.T) {
	var err = xray.New(errors.New("hello world"))
	fmt.Println(err, err.Error())
}
