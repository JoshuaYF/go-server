// Copyright 2019 Axetroy. All rights reserved. MIT license.
package auth

import (
	"errors"
	"github.com/axetroy/go-server/core/controller"
	"github.com/axetroy/go-server/core/controller/wallet"
	"github.com/axetroy/go-server/core/exception"
	"github.com/axetroy/go-server/core/helper"
	"github.com/axetroy/go-server/core/model"
	"github.com/axetroy/go-server/core/schema"
	"github.com/axetroy/go-server/core/service/database"
	"github.com/axetroy/go-server/core/service/redis"
	"github.com/axetroy/go-server/core/service/token"
	"github.com/axetroy/go-server/core/service/wechat"
	"github.com/axetroy/go-server/core/util"
	"github.com/axetroy/go-server/core/validator"
	"github.com/gin-gonic/gin"
	"github.com/jinzhu/gorm"
	"github.com/lib/pq"
	"github.com/mitchellh/mapstructure"
	"net/http"
	"time"
)

type SignInParams struct {
	Account  string `json:"account" valid:"required~请输入登陆账号"`
	Password string `json:"password" valid:"required~请输入密码"`
}

type SignInWithEmailParams struct {
	Email string `json:"email" valid:"required~请输入邮箱"`
	Code  string `json:"code" valid:"required~请输入验证码"`
}

type SignInWithPhoneParams struct {
	Phone string `json:"phone" valid:"required~请输入手机号"`
	Code  string `json:"code" valid:"required~请输入验证码"`
}

type SignInWithWechatParams struct {
	Code string `json:"code" valid:"required~请输入微信授权代码"` // 微信小程序授权之后返回的 code
}

type SignInWithOAuthParams struct {
	Code string `json:"code" valid:"required~请输入授权代码"` // oAuth 授权之后回调返回的 code
}

type WechatCompleteParams struct {
	Code     string  `json:"code" valid:"required~请输入微信授权代码"` // 微信小程序授权之后返回的 code
	Phone    *string `json:"phone"`                           // 手机号
	Username *string `json:"username"`                        // 用户名
}

// 普通帐号登陆
func SignIn(c controller.Context, input SignInParams) (res schema.Response) {
	var (
		err  error
		data = &schema.ProfileWithToken{}
		tx   *gorm.DB
	)

	defer func() {
		if r := recover(); r != nil {
			switch t := r.(type) {
			case string:
				err = errors.New(t)
			case error:
				err = t
			default:
				err = exception.Unknown
			}
		}

		if tx != nil {
			if err != nil {
				_ = tx.Rollback().Error
			} else {
				err = tx.Commit().Error
			}
		}

		helper.Response(&res, data, err)
	}()

	if err = validator.ValidateStruct(input); err != nil {
		return
	}

	userInfo := model.User{
		Password: util.GeneratePassword(input.Password),
	}

	if validator.IsPhone(input.Account) {
		// 用手机号登陆
		userInfo.Phone = &input.Account
	} else if validator.IsEmail(input.Account) {
		// 用邮箱登陆
		userInfo.Email = &input.Account
	} else {
		// 用用户名
		userInfo.Username = input.Account
	}

	tx = database.Db.Begin()

	if err = tx.Where(&userInfo).Preload("Wechat").Last(&userInfo).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			err = exception.InvalidAccountOrPassword
		}
		return
	}

	if err = userInfo.CheckStatusValid(); err != nil {
		return
	}

	if err = mapstructure.Decode(userInfo, &data.ProfilePure); err != nil {
		return
	}

	if userInfo.WechatOpenID != nil {
		if err = mapstructure.Decode(userInfo.Wechat, &data.Wechat); err != nil {
			return
		}
	}

	data.PayPassword = userInfo.PayPassword != nil && len(*userInfo.PayPassword) != 0
	data.CreatedAt = userInfo.CreatedAt.Format(time.RFC3339Nano)
	data.UpdatedAt = userInfo.UpdatedAt.Format(time.RFC3339Nano)

	// generate token
	if t, er := token.Generate(userInfo.Id, false); er != nil {
		err = er
		return
	} else {
		data.Token = t
	}

	// 写入登陆记录
	log := model.LoginLog{
		Uid:     userInfo.Id,                       // 用户ID
		Type:    model.LoginLogTypeUserName,        // 默认用户名登陆
		Command: model.LoginLogCommandLoginSuccess, // 登陆成功
		Client:  c.UserAgent,                       // 用户的 userAgent
		LastIp:  c.Ip,                              // 用户的IP
	}

	if err = tx.Create(&log).Error; err != nil {
		return
	}

	return
}

// 邮箱 + 验证码登陆
func SignInWithEmail(c controller.Context, input SignInWithEmailParams) (res schema.Response) {
	var (
		err  error
		data = &schema.ProfileWithToken{}
		tx   *gorm.DB
	)

	defer func() {
		if r := recover(); r != nil {
			switch t := r.(type) {
			case string:
				err = errors.New(t)
			case error:
				err = t
			default:
				err = exception.Unknown
			}
		}

		if tx != nil {
			if err != nil {
				_ = tx.Rollback().Error
			} else {
				err = tx.Commit().Error
			}
		}

		helper.Response(&res, data, err)
	}()

	// 参数校验
	if err = validator.ValidateStruct(input); err != nil {
		return
	}

	email, err := redis.ClientAuthEmailCode.Get(input.Code).Result()

	// 校验验证码是否正确
	if err != nil || email != input.Email {
		err = exception.InvalidParams
	}

	userInfo := model.User{
		Email: &input.Email,
	}

	tx = database.Db.Begin()

	if err = tx.Where(&userInfo).Preload("Wechat").Last(&userInfo).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			err = exception.InvalidAccountOrPassword
		}
		return
	}

	if err = userInfo.CheckStatusValid(); err != nil {
		return
	}

	if err = mapstructure.Decode(userInfo, &data.ProfilePure); err != nil {
		return
	}

	if userInfo.WechatOpenID != nil {
		if err = mapstructure.Decode(userInfo.Wechat, &data.Wechat); err != nil {
			return
		}
	}

	data.PayPassword = userInfo.PayPassword != nil && len(*userInfo.PayPassword) != 0
	data.CreatedAt = userInfo.CreatedAt.Format(time.RFC3339Nano)
	data.UpdatedAt = userInfo.UpdatedAt.Format(time.RFC3339Nano)

	// generate token
	if t, er := token.Generate(userInfo.Id, false); er != nil {
		err = er
		return
	} else {
		data.Token = t
	}

	// 写入登陆记录
	log := model.LoginLog{
		Uid:     userInfo.Id,                       // 用户ID
		Type:    model.LoginLogTypeUserName,        // 默认用户名登陆
		Command: model.LoginLogCommandLoginSuccess, // 登陆成功
		Client:  c.UserAgent,                       // 用户的 userAgent
		LastIp:  c.Ip,                              // 用户的IP
	}

	if err = tx.Create(&log).Error; err != nil {
		return
	}

	return
}

// 手机 + 验证码登陆
func SignInWithPhone(c controller.Context, input SignInWithPhoneParams) (res schema.Response) {
	var (
		err  error
		data = &schema.ProfileWithToken{}
		tx   *gorm.DB
	)

	defer func() {
		if r := recover(); r != nil {
			switch t := r.(type) {
			case string:
				err = errors.New(t)
			case error:
				err = t
			default:
				err = exception.Unknown
			}
		}

		if tx != nil {
			if err != nil {
				_ = tx.Rollback().Error
			} else {
				err = tx.Commit().Error
			}
		}

		helper.Response(&res, data, err)
	}()

	// 参数校验
	if err = validator.ValidateStruct(input); err != nil {
		return
	}

	phone, err := redis.ClientAuthPhoneCode.Get(input.Code).Result()

	// 校验验证码是否正确
	if err != nil || phone != input.Phone {
		err = exception.InvalidParams
	}

	userInfo := model.User{
		Email: &input.Phone,
	}

	tx = database.Db.Begin()

	if err = tx.Where(&userInfo).Preload("Wechat").Last(&userInfo).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			err = exception.InvalidAccountOrPassword
		}
		return
	}

	if err = userInfo.CheckStatusValid(); err != nil {
		return
	}

	if err = mapstructure.Decode(userInfo, &data.ProfilePure); err != nil {
		return
	}

	if userInfo.WechatOpenID != nil {
		if err = mapstructure.Decode(userInfo.Wechat, &data.Wechat); err != nil {
			return
		}
	}

	data.PayPassword = userInfo.PayPassword != nil && len(*userInfo.PayPassword) != 0
	data.CreatedAt = userInfo.CreatedAt.Format(time.RFC3339Nano)
	data.UpdatedAt = userInfo.UpdatedAt.Format(time.RFC3339Nano)

	// generate token
	if t, er := token.Generate(userInfo.Id, false); er != nil {
		err = er
		return
	} else {
		data.Token = t
	}

	// 写入登陆记录
	log := model.LoginLog{
		Uid:     userInfo.Id,                       // 用户ID
		Type:    model.LoginLogTypeUserName,        // 默认用户名登陆
		Command: model.LoginLogCommandLoginSuccess, // 登陆成功
		Client:  c.UserAgent,                       // 用户的 userAgent
		LastIp:  c.Ip,                              // 用户的IP
	}

	if err = tx.Create(&log).Error; err != nil {
		return
	}

	return
}

// 使用微信小程序登陆
func SignInWithWechat(c controller.Context, input SignInWithWechatParams) (res schema.Response) {
	var (
		err  error
		data = &schema.ProfileWithToken{}
		tx   *gorm.DB
	)

	defer func() {
		if r := recover(); r != nil {
			switch t := r.(type) {
			case string:
				err = errors.New(t)
			case error:
				err = t
			default:
				err = exception.Unknown
			}
		}

		if tx != nil {
			if err != nil {
				_ = tx.Rollback().Error
			} else {
				err = tx.Commit().Error
			}
		}

		helper.Response(&res, data, err)
	}()

	// 参数校验
	if err = validator.ValidateStruct(input); err != nil {
		return
	}

	wechatInfo, wechatErr := wechat.FetchOpenID(input.Code)

	if wechatErr != nil {
		err = wechatErr
		return
	}

	tx = database.Db.Begin()

	wechatOpenID := model.WechatOpenID{
		Id: wechatInfo.OpenID,
	}

	// 去查表
	result := tx.Where(&wechatOpenID).Preload("User").First(&wechatOpenID)

	var userInfo *model.User

	if result.RecordNotFound() {
		var (
			uid      = util.GenerateId()
			username = "v" + uid
		)

		userInfo = &model.User{
			Username:                username,
			Nickname:                &username,
			Password:                util.GeneratePassword(uid),
			Status:                  model.UserStatusInit,
			Role:                    pq.StringArray{model.DefaultUser.Name},
			Gender:                  model.GenderUnknown,
			WechatOpenID:            &wechatOpenID.Id,
			UsernameRenameRemaining: 1, // 允许微信注册的用户可以重命名一次
		}

		if err = tx.Create(userInfo).Error; err != nil {
			return
		}

		if err = tx.Create(&model.WechatOpenID{
			Id:  wechatInfo.OpenID,
			Uid: userInfo.Id,
		}).Error; err != nil {
			return
		}

		// 创建用户对应的钱包账号
		for _, walletName := range model.Wallets {
			if err = tx.Table(wallet.GetTableName(walletName)).Create(&model.Wallet{
				Id:       userInfo.Id,
				Currency: walletName,
				Balance:  0,
				Frozen:   0,
			}).Error; err != nil {
				return
			}
		}

	} else {
		userInfo = &wechatOpenID.User
	}

	if userInfo == nil {
		err = exception.NoData
		return
	}

	if err = mapstructure.Decode(userInfo, &data.ProfilePure); err != nil {
		return
	}

	wechatBindingInfo := schema.WechatBindingInfo{}

	if err = mapstructure.Decode(wechatOpenID, &wechatBindingInfo); err != nil {
		return
	}

	data.Wechat = &wechatBindingInfo
	data.PayPassword = userInfo.PayPassword != nil && len(*userInfo.PayPassword) != 0
	data.CreatedAt = userInfo.CreatedAt.Format(time.RFC3339Nano)
	data.UpdatedAt = userInfo.UpdatedAt.Format(time.RFC3339Nano)

	// generate token
	if t, er := token.Generate(userInfo.Id, false); er != nil {
		err = er
		return
	} else {
		data.Token = t
	}

	// 写入登陆记录
	log := model.LoginLog{
		Uid:     userInfo.Id,                       // 用户ID
		Type:    model.LoginLogTypeWechat,          // 微信登陆
		Command: model.LoginLogCommandLoginSuccess, // 登陆成功
		Client:  c.UserAgent,                       // 用户的 userAgent
		LastIp:  c.Ip,                              // 用户的IP
	}

	if err = tx.Create(&log).Error; err != nil {
		return
	}

	return
}

// 使用 oAuth 认证方式登陆
func SignInWithOAuth(c controller.Context, input SignInWithOAuthParams) (res schema.Response) {
	var (
		err  error
		data = &schema.ProfileWithToken{}
		tx   *gorm.DB
	)

	defer func() {
		if r := recover(); r != nil {
			switch t := r.(type) {
			case string:
				err = errors.New(t)
			case error:
				err = t
			default:
				err = exception.Unknown
			}
		}

		if tx != nil {
			if err != nil {
				_ = tx.Rollback().Error
			} else {
				err = tx.Commit().Error
			}
		}

		helper.Response(&res, data, err)
	}()

	// 参数校验
	if err = validator.ValidateStruct(input); err != nil {
		return
	}

	uid, err := redis.ClientOAuthCode.Get(input.Code).Result()

	if err != nil {
		return
	}

	var userInfo = model.User{
		Id: uid,
	}

	if err = tx.Where(&userInfo).Preload("Wechat").Find(&userInfo).Error; err != nil {
		return
	}

	if err = mapstructure.Decode(userInfo, &data.ProfilePure); err != nil {
		return
	}

	if userInfo.WechatOpenID != nil {
		wechatBindingInfo := schema.WechatBindingInfo{}

		if err = mapstructure.Decode(userInfo.Wechat, &wechatBindingInfo); err != nil {
			return
		}

		data.Wechat = &wechatBindingInfo
	}

	// generate token
	if t, er := token.Generate(userInfo.Id, false); er != nil {
		err = er
		return
	} else {
		data.Token = t
	}

	// 写入登陆记录
	log := model.LoginLog{
		Uid:     userInfo.Id,                       // 用户ID
		Type:    model.LoginLogTypeUserName,        // 默认用户名登陆
		Command: model.LoginLogCommandLoginSuccess, // 登陆成功
		Client:  c.UserAgent,                       // 用户的 userAgent
		LastIp:  c.Ip,                              // 用户的IP
	}

	if err = tx.Create(&log).Error; err != nil {
		return
	}

	data.PayPassword = userInfo.PayPassword != nil && len(*userInfo.PayPassword) != 0
	data.CreatedAt = userInfo.CreatedAt.Format(time.RFC3339Nano)
	data.UpdatedAt = userInfo.UpdatedAt.Format(time.RFC3339Nano)

	return
}

func SignInRouter(c *gin.Context) {
	var (
		input SignInParams
		err   error
		res   = schema.Response{}
	)

	defer func() {
		if err != nil {
			res.Data = nil
			res.Message = err.Error()
		}
		c.JSON(http.StatusOK, res)
	}()

	if err = c.ShouldBindJSON(&input); err != nil {
		err = exception.InvalidParams
		return
	}

	res = SignIn(controller.NewContext(c), input)
}

func SignInWithEmailRouter(c *gin.Context) {
	var (
		input SignInWithEmailParams
		err   error
		res   = schema.Response{}
	)

	defer func() {
		if err != nil {
			res.Data = nil
			res.Message = err.Error()
		}
		c.JSON(http.StatusOK, res)
	}()

	if err = c.ShouldBindJSON(&input); err != nil {
		err = exception.InvalidParams
		return
	}

	res = SignInWithEmail(controller.NewContext(c), input)
}

func SignInWithPhoneRouter(c *gin.Context) {
	var (
		input SignInWithPhoneParams
		err   error
		res   = schema.Response{}
	)

	defer func() {
		if err != nil {
			res.Data = nil
			res.Message = err.Error()
		}
		c.JSON(http.StatusOK, res)
	}()

	if err = c.ShouldBindJSON(&input); err != nil {
		err = exception.InvalidParams
		return
	}

	res = SignInWithPhone(controller.NewContext(c), input)
}

func SignInWithWechatRouter(c *gin.Context) {
	var (
		input SignInWithWechatParams
		err   error
		res   = schema.Response{}
	)

	defer func() {
		if err != nil {
			res.Data = nil
			res.Message = err.Error()
		}
		c.JSON(http.StatusOK, res)
	}()

	if err = c.ShouldBindJSON(&input); err != nil {
		err = exception.InvalidParams
		return
	}

	res = SignInWithWechat(controller.NewContext(c), input)
}

func SignInWithOAuthRouter(c *gin.Context) {
	var (
		input SignInWithOAuthParams
		err   error
		res   = schema.Response{}
	)

	defer func() {
		if err != nil {
			res.Data = nil
			res.Message = err.Error()
		}
		c.JSON(http.StatusOK, res)
	}()

	if err = c.ShouldBindJSON(&input); err != nil {
		err = exception.InvalidParams
		return
	}

	res = SignInWithOAuth(controller.NewContext(c), input)
}
