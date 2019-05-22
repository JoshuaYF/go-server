package src

import (
	"github.com/axetroy/go-server/src/controller/address"
	"github.com/axetroy/go-server/src/controller/auth"
	"github.com/axetroy/go-server/src/controller/banner"
	"github.com/axetroy/go-server/src/controller/downloader"
	"github.com/axetroy/go-server/src/controller/email"
	"github.com/axetroy/go-server/src/controller/finance"
	"github.com/axetroy/go-server/src/controller/invite"
	"github.com/axetroy/go-server/src/controller/message"
	"github.com/axetroy/go-server/src/controller/news"
	"github.com/axetroy/go-server/src/controller/notification"
	"github.com/axetroy/go-server/src/controller/oauth2"
	"github.com/axetroy/go-server/src/controller/resource"
	"github.com/axetroy/go-server/src/controller/static"
	"github.com/axetroy/go-server/src/controller/transfer"
	"github.com/axetroy/go-server/src/controller/uploader"
	"github.com/axetroy/go-server/src/controller/user"
	"github.com/axetroy/go-server/src/controller/wallet"
	"github.com/axetroy/go-server/src/middleware"
	"github.com/gin-gonic/gin"
	"net/http"
)

var RouterUserClient *gin.Engine

func init() {
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	router.Use(gin.Logger())

	// Recovery middleware recovers from any panics and writes a 500 if there was one.
	router.Use(gin.Recovery())

	router.GET("/ping", func(context *gin.Context) {
		context.String(http.StatusOK, "pong")
	})

	// Simple group: v1
	v1 := router.Group("/v1")

	v1.Use(middleware.Common)

	{
		v1.GET("", func(context *gin.Context) {
			context.JSON(http.StatusOK, gin.H{"ping": "pong"})
		})

		userAuthMiddleware := middleware.Authenticate(false) // 用户Token的中间件

		// 认证类
		authRouter := v1.Group("/auth")
		{
			authRouter.POST("/signup", auth.SignUpRouter)               // 注册账号
			authRouter.POST("/signin", auth.SignInRouter)               // 登陆账号
			authRouter.POST("/activation", auth.ActivationRouter)       // 激活账号
			authRouter.PUT("/password/reset", auth.ResetPasswordRouter) // 密码重置

		}

		oauthRouter := v1.Group("/oauth2")
		{
			oauthRouter.GET("/google", oauth2.GoogleLoginRouter)             // 用 Google 登陆
			oauthRouter.GET("/google_callback", oauth2.GoogleCallbackRouter) // Google 认证完成后跳转到这里，用户不应该访问这个地址
		}

		// 用户类
		userRouter := v1.Group("/user")
		{
			userRouter.Use(userAuthMiddleware)
			userRouter.GET("/signout", user.SignOut)                             // 用户登出
			userRouter.GET("/profile", user.GetProfileRouter)                    // 获取用户详细信息
			userRouter.PUT("/profile", user.UpdateProfileRouter)                 // 更新用户资料
			userRouter.PUT("/password", user.UpdatePasswordRouter)               // 更新登陆密码
			userRouter.POST("/password2", user.SetPayPasswordRouter)             // 设置交易密码
			userRouter.PUT("/password2", user.UpdatePayPasswordRouter)           // 更新交易密码
			userRouter.PUT("/password2/reset", user.ResetPayPasswordRouter)      // 重置交易密码
			userRouter.POST("/password2/reset", user.SendResetPayPasswordRouter) // 发送重置交易密码的邮件/短信
			userRouter.POST("/avatar", user.UploadAvatarRouter)                  // 上传用户头像
			// 邀请人列表
			inviteRouter := userRouter.Group("/invite")
			{
				inviteRouter.GET("", invite.GetListRouter)          // 获取我已邀请的列表
				inviteRouter.GET("/i/:invite_id", invite.GetRouter) // 获取单条邀请记录详情
			}
			// 收货地址
			addressRouter := userRouter.Group("/address")
			{
				addressRouter.GET("", address.GetListRouter)                 // 获取地址列表
				addressRouter.POST("", address.CreateRouter)                 // 添加收货地址
				addressRouter.PUT("/a/:address_id", address.UpdateRouter)    // 更新收货地址
				addressRouter.DELETE("/a/:address_id", address.DeleteRouter) // 删除收货地址
				addressRouter.GET("/a/:address_id", address.GetDetailRouter) // 获取地址详情
				addressRouter.GET("/default", address.GetDefaultRouter)      // 获取默认地址
			}
		}

		// 钱包类
		walletRouter := v1.Group("/wallet")
		{
			walletRouter.Use(userAuthMiddleware)
			walletRouter.GET("/map", wallet.GetWalletsRouter)        // 获取我的钱包map对象
			walletRouter.GET("/w/:currency", wallet.GetWalletRouter) // 获取单个钱包的详细信息

		}

		transferRouter := v1.Group("/transfer")
		{
			transferRouter.Use(userAuthMiddleware)
			transferRouter.GET("", transfer.GetHistoryRouter)                      // 获取我的转账记录
			transferRouter.POST("", middleware.AuthPayPassword, transfer.ToRouter) // 转账给某人
			transferRouter.GET("/t/:transfer_id", transfer.GetDetailRouter)        // 获取单条转账详情
		}

		// 财务日志
		financeRouter := v1.Group("/finance")
		{
			financeRouter.Use(userAuthMiddleware)
			financeRouter.GET("/history", finance.GetHistory) // TODO: 获取我的财务日志
		}

		// 新闻咨询类
		newsRouter := v1.Group("/news")
		{
			newsRouter.GET("", news.GetListRouter)       // 获取新闻公告列表
			newsRouter.GET("/n/:id", news.GetNewsRouter) // 获取单个新闻公告详情
		}

		// 系统通知
		notificationRouter := v1.Group("/notification")
		{
			notificationRouter.Use(userAuthMiddleware)
			notificationRouter.GET("", notification.GetListUserRouter)     // 获取系统通知列表
			notificationRouter.GET("/n/:id", notification.GetRouter)       // 获取某一条系统通知详情
			notificationRouter.PUT("/n/:id/read", notification.ReadRouter) // 标记通知为已读
		}

		// 用户的个人消息, 个人消息是可以删除的
		messageRouter := v1.Group("/message")
		{
			messageRouter.Use(userAuthMiddleware)
			messageRouter.GET("", message.GetListRouter)                       // 获取我的消息列表
			messageRouter.GET("/m/:message_id", message.GetRouter)             // 获取单个消息详情
			messageRouter.PUT("/m/:message_id/read", message.ReadRouter)       // 标记消息为已读
			messageRouter.DELETE("/m/:message_id", message.DeleteByUserRouter) // 删除消息
		}

		// Banner
		bannerRouter := v1.Group("banner")
		{
			bannerRouter.GET("", banner.GetListRouter)                // 获取 banner 列表
			bannerRouter.GET("/b/:banner_id", banner.GetBannerRouter) // 获取 banner 详情
		}

		// 通用类
		{
			// 邮件服务
			v1.POST("/email/send/activation", email.SendActivationEmailRouter)        // 发送激活邮件
			v1.POST("/email/send/password/reset", email.SendResetPasswordEmailRouter) // 发送密码重置邮件

			// 文件上传 (需要验证token)
			uploadRouter := v1.Group("/upload")
			{
				uploadRouter.Use(userAuthMiddleware)
				uploadRouter.POST("/file", uploader.File)   // 上传文件
				uploadRouter.POST("/image", uploader.Image) // 上传图片
			}
			// 单纯获取资源文本
			v1.GET("/resource/file/:filename", resource.File)
			v1.GET("/resource/image/:filename", resource.Image)
			v1.GET("/resource/thumbnail/:filename", resource.Thumbnail)
			// 下载资源
			v1.GET("/download/file/:filename", downloader.File)
			v1.GET("/download/image/:filename", downloader.Image)
			v1.GET("/download/thumbnail/:filename", downloader.Thumbnail)
			// 公共资源目录
			v1.GET("/public/:filename", static.Get)
			v1.GET("/avatar/:filename", user.GetAvatarRouter) // 获取用户头像

			v1.GET("/area", address.AreaListRouter) // 获取地址选择列表
		}
	}

	RouterUserClient = router
}
