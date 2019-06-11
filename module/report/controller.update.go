// Copyright 2019 Axetroy. All rights reserved. MIT license.
package report

import (
	"errors"
	"github.com/asaskevich/govalidator"
	"github.com/axetroy/go-server/exception"
	"github.com/axetroy/go-server/middleware"
	"github.com/axetroy/go-server/module/report/report_model"
	"github.com/axetroy/go-server/module/report/report_schema"
	"github.com/axetroy/go-server/schema"
	"github.com/axetroy/go-server/service/database"
	"github.com/gin-gonic/gin"
	"github.com/jinzhu/gorm"
	"github.com/mitchellh/mapstructure"
	"net/http"
	"time"
)

type UpdateParams struct {
	Status *report_model.ReportStatus `json:"status" valid:"required~请选择要标记的状态"`
}

type UpdateByAdminParams struct {
	UpdateParams
	Locked *bool `json:"locked"` // 是否锁定
}

func Update(context schema.Context, reportId string, input UpdateParams) (res schema.Response) {
	var (
		err          error
		data         report_schema.Report
		tx           *gorm.DB
		isValidInput bool
		shouldUpdate bool
	)

	defer func() {
		if r := recover(); r != nil {
			switch t := r.(type) {
			case string:
				err = errors.New(t)
			case error:
				err = t
			default:
				err = exception.ErrUnknown
			}
		}

		if tx != nil {
			if err != nil || !shouldUpdate {
				_ = tx.Rollback().Error
			} else {
				err = tx.Commit().Error
			}
		}

		if err != nil {
			res.Data = nil
			res.Message = err.Error()
		} else {
			res.Data = data
			res.Status = schema.StatusSuccess
		}
	}()

	// 参数校验
	if isValidInput, err = govalidator.ValidateStruct(input); err != nil {
		return
	} else if isValidInput == false {
		err = exception.ErrInvalidParams
		return
	}

	tx = database.Db.Begin()

	reportInfo := report_model.Report{
		Id:  reportId,
		Uid: context.Uid,
	}

	if err = tx.First(&reportInfo).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			err = exception.ErrNoData
			return
		}
		return
	}

	// 如果已被锁定，则无法更新状态
	if reportInfo.Locked {
		err = errors.New("该反馈已被锁定, 无法更新")
		return
	}

	updatedModel := report_model.Report{}

	if input.Status != nil {
		// 状态不能重复改变, 忽略本次操作.
		if reportInfo.Status == *input.Status {
			return
		}
		updatedModel.Status = *input.Status
		shouldUpdate = true
	}

	if shouldUpdate == false {
		return
	}

	if err = tx.Model(&reportInfo).Where(&report_model.Report{
		Id:  reportId,
		Uid: context.Uid,
	}).Update(updatedModel).Error; err != nil {
		return
	}

	if err = mapstructure.Decode(reportInfo, &data.ReportPure); err != nil {
		return
	}

	data.CreatedAt = reportInfo.CreatedAt.Format(time.RFC3339Nano)
	data.UpdatedAt = reportInfo.UpdatedAt.Format(time.RFC3339Nano)

	return
}

func UpdateRouter(ctx *gin.Context) {
	var (
		input UpdateParams
		err   error
		res   = schema.Response{}
	)

	defer func() {
		if err != nil {
			res.Data = nil
			res.Message = err.Error()
		}
		ctx.JSON(http.StatusOK, res)
	}()

	reportId := ctx.Param("report_id")

	if err = ctx.ShouldBindJSON(&input); err != nil {
		err = exception.ErrInvalidParams
		return
	}

	res = Update(schema.Context{
		Uid: ctx.GetString(middleware.ContextUidField),
	}, reportId, input)
}

func UpdateByAdmin(context schema.Context, reportId string, input UpdateByAdminParams) (res schema.Response) {
	var (
		err          error
		data         report_schema.Report
		tx           *gorm.DB
		isValidInput bool
		shouldUpdate bool
	)

	defer func() {
		if r := recover(); r != nil {
			switch t := r.(type) {
			case string:
				err = errors.New(t)
			case error:
				err = t
			default:
				err = exception.ErrUnknown
			}
		}

		if tx != nil {
			if err != nil || !shouldUpdate {
				_ = tx.Rollback().Error
			} else {
				err = tx.Commit().Error
			}
		}

		if err != nil {
			res.Data = nil
			res.Message = err.Error()
		} else {
			res.Data = data
			res.Status = schema.StatusSuccess
		}
	}()

	// 参数校验
	if isValidInput, err = govalidator.ValidateStruct(input); err != nil {
		return
	} else if isValidInput == false {
		err = exception.ErrInvalidParams
		return
	}

	tx = database.Db.Begin()

	reportInfo := report_model.Report{
		Id: reportId,
	}

	if err = tx.First(&reportInfo).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			err = exception.ErrNoData
			return
		}
		return
	}

	updatedModel := report_model.Report{}

	if input.Status != nil {
		updatedModel.Status = *input.Status
		shouldUpdate = true
	}

	if input.Locked != nil {
		updatedModel.Locked = *input.Locked
		shouldUpdate = true
	}

	if shouldUpdate == false {
		return
	}

	if err = tx.Model(&reportInfo).Where(&report_model.Report{
		Id: reportId,
	}).Update(updatedModel).Error; err != nil {
		return
	}

	if err = mapstructure.Decode(reportInfo, &data.ReportPure); err != nil {
		return
	}

	data.CreatedAt = reportInfo.CreatedAt.Format(time.RFC3339Nano)
	data.UpdatedAt = reportInfo.UpdatedAt.Format(time.RFC3339Nano)

	return
}

func UpdateByAdminRouter(ctx *gin.Context) {
	var (
		input UpdateByAdminParams
		err   error
		res   = schema.Response{}
	)

	defer func() {
		if err != nil {
			res.Data = nil
			res.Message = err.Error()
		}
		ctx.JSON(http.StatusOK, res)
	}()

	reportId := ctx.Param("report_id")

	if err = ctx.ShouldBindJSON(&input); err != nil {
		err = exception.ErrInvalidParams
		return
	}

	res = UpdateByAdmin(schema.Context{
		Uid: ctx.GetString(middleware.ContextUidField),
	}, reportId, input)
}
