// Copyright 2019 Axetroy. All rights reserved. MIT license.
package email_test

import (
	"encoding/json"
	"github.com/axetroy/go-server/core/controller/auth"
	"github.com/axetroy/go-server/core/controller/email"
	"github.com/axetroy/go-server/core/exception"
	"github.com/axetroy/go-server/core/schema"
	"github.com/axetroy/go-server/core/service/captcha"
	"github.com/axetroy/go-server/tester"
	"github.com/stretchr/testify/assert"
	"net/http"
	"testing"
)

func TestGenerateResetCode(t *testing.T) {
	user, _ := tester.CreateUser()

	defer auth.DeleteUserByUserName(user.Username)

	code := captcha.GenerateResetCode(user.Id)

	assert.IsType(t, "", code)
	assert.NotEmpty(t, code)
}

func TestSendResetPasswordEmail(t *testing.T) {

	body, _ := json.Marshal(&email.SendResetPasswordEmailParams{
		Email: "123adsd@dasdad.com", // invalid email
	})

	r := tester.HttpUser.Post("/v1/email/send/password/reset", body, nil)

	assert.Equal(t, http.StatusOK, r.Code)

	res := schema.Response{}

	assert.Nil(t, json.Unmarshal(r.Body.Bytes(), &res))

	assert.Equal(t, exception.UserNotExist.Code(), res.Status)
	assert.Equal(t, exception.UserNotExist.Error(), res.Message)
}
