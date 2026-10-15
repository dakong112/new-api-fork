package router

import (
	"net/http"

	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/service/authz"

	// Import oauth package to register providers via init()
	_ "github.com/QuantumNous/new-api/oauth"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
)

func SetApiRouter(router *gin.Engine) {
	apiRouter := router.Group("/api")
	apiRouter.Use(middleware.RouteTag("api"))
	apiRouter.Use(gzip.Gzip(gzip.DefaultCompression))
	apiRouter.Use(middleware.AccessTokenAudit())
	apiRouter.Use(middleware.BodyStorageCleanup()) // 清理请求体存储
	apiRouter.Use(middleware.GlobalAPIRateLimit())
	anonymousRequestBodyLimit := middleware.AnonymousRequestBodyLimit()
	{
		apiRouter.GET("/setup", controller.GetSetup)
		apiRouter.POST("/setup", anonymousRequestBodyLimit, controller.PostSetup)
		apiRouter.GET("/status", middleware.SSOClientCORS(), controller.GetStatus)
		apiRouter.GET("/uptime/status", controller.GetUptimeKumaStatus)
		apiRouter.GET("/models", middleware.UserAuth(), controller.DashboardListModels)
		handlePermissionRoute(apiRouter.Group("", middleware.AdminAuth()), http.MethodGet, "/status/test", authz.LogRead, controller.TestStatus)
		apiRouter.GET("/notice", controller.GetNotice)
		apiRouter.GET("/user-agreement", controller.GetUserAgreement)
		apiRouter.GET("/privacy-policy", controller.GetPrivacyPolicy)
		apiRouter.GET("/about", controller.GetAbout)
		//apiRouter.GET("/midjourney", controller.GetMidjourney)
		apiRouter.GET("/home_page_content", controller.GetHomePageContent)
		apiRouter.GET("/pricing", middleware.HeaderNavModuleAuth("pricing"), controller.GetPricing)
		perfMetricsRoute := apiRouter.Group("/perf-metrics")
		perfMetricsRoute.Use(middleware.HeaderNavModulePublicOrUserAuth("pricing"))
		{
			perfMetricsRoute.GET("/summary", controller.GetPerfMetricsSummary)
			perfMetricsRoute.GET("", controller.GetPerfMetrics)
		}
		apiRouter.GET("/rankings", middleware.HeaderNavModuleAuth("rankings"), controller.GetRankings)
		apiRouter.GET("/verification", middleware.SSOClientCORS(), middleware.EmailVerificationRateLimit(), middleware.TurnstileCheck(), controller.SendEmailVerification)
		apiRouter.GET("/reset_password", middleware.CriticalRateLimit(), middleware.TurnstileCheck(), controller.SendPasswordResetEmail)
		apiRouter.POST("/user/reset", middleware.CriticalRateLimit(), anonymousRequestBodyLimit, controller.ResetPassword)
		// OAuth routes - specific routes must come before :provider wildcard
		apiRouter.POST("/oauth/state", middleware.CriticalRateLimit(), middleware.DisableCache(), middleware.TryUserAuth(), anonymousRequestBodyLimit, controller.GenerateOAuthCode)
		apiRouter.POST("/oauth/email/bind/start", middleware.UserAuth(), middleware.CriticalRateLimit(), middleware.UserCriticalRateLimit("account-security"), middleware.EmailVerificationRateLimit(), middleware.DisableCache(), controller.EmailBindStart)
		apiRouter.POST("/oauth/email/bind/resend", middleware.UserAuth(), middleware.CriticalRateLimit(), middleware.UserCriticalRateLimit("account-security"), middleware.EmailVerificationRateLimit(), middleware.DisableCache(), controller.EmailBindResend)
		apiRouter.POST("/oauth/email/bind", middleware.UserAuth(), middleware.CriticalRateLimit(), middleware.UserCriticalRateLimit("account-security"), middleware.DisableCache(), controller.EmailBind)
		// WeChat uses its existing authorization-code service.
		apiRouter.GET("/oauth/wechat", middleware.CriticalRateLimit(), middleware.DisableCache(), controller.WeChatAuth)
		apiRouter.POST("/oauth/wechat/bind", middleware.UserAuth(), middleware.CriticalRateLimit(), controller.WeChatBind)
		apiRouter.GET("/oauth/telegram/login", middleware.CriticalRateLimit(), middleware.DisableCache(), controller.TelegramLegacyAuth)
		apiRouter.POST("/oauth/telegram/bind/start", middleware.UserAuth(), middleware.CriticalRateLimit(), middleware.DisableCache(), controller.TelegramLegacyAuth)
		apiRouter.GET("/oauth/telegram/bind/:flow_token", middleware.CriticalRateLimit(), middleware.DisableCache(), controller.TelegramLegacyAuth)
		// Standard OAuth providers (GitHub, Discord, OIDC, LinuxDO, Telegram) - unified route
		apiRouter.GET("/oauth/:provider", middleware.CriticalRateLimit(), middleware.DisableCache(), middleware.TryUserAuth(), controller.HandleOAuth)
		apiRouter.GET("/ratio_config", middleware.CriticalRateLimit(), controller.GetRatioConfig)

		apiRouter.POST("/stripe/webhook", anonymousRequestBodyLimit, controller.StripeWebhook)
		apiRouter.POST("/creem/webhook", anonymousRequestBodyLimit, controller.CreemWebhook)
		apiRouter.POST("/waffo/webhook", anonymousRequestBodyLimit, controller.WaffoWebhook)
		// :env separates test vs prod URLs so the operator can register each
		// in Pancake's matching webhook slot; handler enforces env match.
		apiRouter.POST("/waffo-pancake/webhook/:env", anonymousRequestBodyLimit, controller.WaffoPancakeWebhook)

		// Universal secure verification routes
		apiRouter.GET("/verify/methods", middleware.UserAuth(), middleware.DisableCache(), controller.GetVerificationMethods)
		apiRouter.POST("/verify", middleware.UserAuth(), middleware.CriticalRateLimit(), middleware.UserCriticalRateLimit("security-verification"), middleware.DisableCache(), controller.UniversalVerify)

		// Browser pages of registered SSO applications call these cross-origin.
		for _, path := range []string{"/status", "/verification", "/sso/code", "/user/register", "/user/login/encryption-key", "/user/login", "/user/login/verify"} {
			apiRouter.OPTIONS(path, middleware.SSOClientCORS())
		}

		ssoRoute := apiRouter.Group("/sso")
		ssoRoute.POST("/code", middleware.SSOClientCORS(), middleware.UserAuth(), middleware.UserCriticalRateLimit("sso-code"), middleware.DisableCache(), controller.CreateSSOCode)
		ssoRoute.GET("/apps", middleware.UserAuth(), middleware.DisableCache(), controller.ListSSOApps)
		ssoRoute.POST("/exchange", middleware.DisableCache(), anonymousRequestBodyLimit, controller.ExchangeSSOCode)
		ssoRoute.POST("/ticket", middleware.UserAuth(), middleware.UserCriticalRateLimit("sso-ticket"), middleware.DisableCache(), anonymousRequestBodyLimit, controller.CreateSSOTicket)
		ssoRoute.POST("/key", middleware.UserAuth(), middleware.UserCriticalRateLimit("sso-key"), middleware.DisableCache(), anonymousRequestBodyLimit, controller.CreateSSOKey)
		ssoRoute.POST("/consume", middleware.UserAuth(), middleware.DisableCache(), anonymousRequestBodyLimit, controller.ConsumeSSOQuota)
		ssoRoute.GET("/client", middleware.DisableCache(), controller.GetSSOClient)
		ssoRoute.POST("/ticket/preview", middleware.SessionCookieOriginGuard(), middleware.SameSiteJSONOnly(), middleware.CriticalRateLimit(), middleware.DisableCache(), anonymousRequestBodyLimit, controller.PreviewSSOTicket)
		ssoRoute.POST("/ticket/redeem", middleware.SessionCookieOriginGuard(), middleware.SameSiteJSONOnly(), middleware.CriticalRateLimit(), middleware.DisableCache(), anonymousRequestBodyLimit, controller.RedeemSSOTicket)

		userRoute := apiRouter.Group("/user")
		{
			userRoute.POST("/auth/refresh", middleware.SessionCookieOriginGuard(), middleware.CriticalRateLimit(), middleware.DisableCache(), controller.RefreshAuth)
			userRoute.POST("/auth/logout", middleware.SessionCookieOriginGuard(), middleware.CriticalRateLimit(), middleware.DisableCache(), controller.AuthLogout)
			userRoute.POST("/register", middleware.SSOClientCORS(), middleware.CriticalRateLimit(), anonymousRequestBodyLimit, middleware.TurnstileCheck(), controller.Register)
			userRoute.GET("/login/encryption-key", middleware.SSOClientCORS(), middleware.DisableCache(), controller.GetPasswordEncryptionKey)
			userRoute.POST("/login", middleware.SSOClientCORS(), middleware.CriticalRateLimit(), middleware.DisableCache(), anonymousRequestBodyLimit, middleware.TurnstileCheck(), controller.Login)
			userRoute.POST("/login/2fa", middleware.CriticalRateLimit(), middleware.DisableCache(), anonymousRequestBodyLimit, controller.Verify2FALogin)
			userRoute.POST("/login/verify", middleware.SSOClientCORS(), middleware.CriticalRateLimit(), middleware.DisableCache(), anonymousRequestBodyLimit, controller.VerifyLogin)
			userRoute.POST("/login/passkey/begin", middleware.CriticalRateLimit(), middleware.DisableCache(), anonymousRequestBodyLimit, controller.LoginPasskeyBegin)
			userRoute.POST("/login/passkey/finish", middleware.CriticalRateLimit(), middleware.DisableCache(), anonymousRequestBodyLimit, controller.LoginPasskeyFinish)
			userRoute.POST("/passkey/login/begin", middleware.CriticalRateLimit(), middleware.DisableCache(), anonymousRequestBodyLimit, controller.PasskeyLoginBegin)
			userRoute.POST("/passkey/login/finish", middleware.CriticalRateLimit(), middleware.DisableCache(), anonymousRequestBodyLimit, controller.PasskeyLoginFinish)
			//userRoute.POST("/tokenlog", middleware.CriticalRateLimit(), controller.TokenLog)
			userRoute.POST("/epay/notify", anonymousRequestBodyLimit, controller.EpayNotify)
			userRoute.GET("/epay/notify", controller.EpayNotify)
			userRoute.GET("/groups", controller.GetUserGroups)

			selfRoute := userRoute.Group("/")
			selfRoute.Use(middleware.DisableCache(), middleware.UserAuth())
			{
				selfRoute.GET("/sessions", middleware.DisableCache(), controller.GetLoginSessions)
				selfRoute.DELETE("/sessions/:sid", middleware.DisableCache(), controller.DeleteLoginSession)
				selfRoute.POST("/sessions/revoke-others", middleware.DisableCache(), controller.RevokeOtherLoginSessions)
				selfRoute.GET("/self/groups", controller.GetUserGroups)
				selfRoute.GET("/self", controller.GetSelf)
				selfRoute.GET("/models", controller.GetUserModels)
				selfRoute.PUT("/self", middleware.CriticalRateLimit(), middleware.DisableCache(), controller.UpdateSelf)
				selfRoute.DELETE("/self", middleware.DisableCache(), controller.DeleteSelf)
				accessTokenRoute := selfRoute.Group("/access_tokens")
				accessTokenRoute.Use(middleware.DisableCache())
				{
					accessTokenRoute.GET("", controller.ListAccessTokens)
					accessTokenRoute.GET("/catalog", controller.GetAccessTokenCatalog)
					accessTokenRoute.GET("/scopes", controller.GetAccessTokenScopes)
					accessTokenRoute.POST("", middleware.CriticalRateLimit(), middleware.UserCriticalRateLimit("access-token"), controller.CreateAccessToken)
					accessTokenRoute.PATCH("/:id", middleware.CriticalRateLimit(), middleware.UserCriticalRateLimit("access-token"), controller.UpdateAccessToken)
					accessTokenRoute.DELETE("/:id", middleware.CriticalRateLimit(), middleware.UserCriticalRateLimit("access-token"), controller.DeleteAccessToken)
					accessTokenRoute.DELETE("/legacy", middleware.CriticalRateLimit(), middleware.UserCriticalRateLimit("access-token"), controller.RevokeLegacyAccessToken)
				}
				selfRoute.GET("/passkey", controller.PasskeyStatus)
				selfRoute.POST("/passkey/register/begin", middleware.UserCriticalRateLimit("security-verification"), middleware.DisableCache(), controller.PasskeyRegisterBegin)
				selfRoute.POST("/passkey/register/finish", middleware.UserCriticalRateLimit("security-verification"), middleware.DisableCache(), controller.PasskeyRegisterFinish)
				selfRoute.POST("/passkey/verify/begin", middleware.UserCriticalRateLimit("security-verification"), middleware.DisableCache(), controller.PasskeyVerifyBegin)
				selfRoute.POST("/passkey/verify/finish", middleware.UserCriticalRateLimit("security-verification"), middleware.DisableCache(), controller.PasskeyVerifyFinish)
				selfRoute.DELETE("/passkey", middleware.DisableCache(), controller.PasskeyDelete)
				selfRoute.GET("/aff", controller.GetAffCode)
				selfRoute.GET("/topup/info", controller.GetTopUpInfo)
				selfRoute.GET("/topup/self", controller.GetUserTopUps)
				selfRoute.POST("/topup", middleware.UserCriticalRateLimit("wallet-topup"), controller.TopUp)
				selfRoute.POST("/pay", middleware.UserCriticalRateLimit("wallet-pay"), controller.RequestEpay)
				selfRoute.POST("/amount", controller.RequestAmount)
				selfRoute.POST("/stripe/pay", middleware.UserCriticalRateLimit("wallet-pay"), controller.RequestStripePay)
				selfRoute.POST("/stripe/amount", controller.RequestStripeAmount)
				selfRoute.POST("/creem/pay", middleware.UserCriticalRateLimit("wallet-pay"), controller.RequestCreemPay)
				selfRoute.POST("/waffo/amount", controller.RequestWaffoAmount)
				selfRoute.POST("/waffo/pay", middleware.UserCriticalRateLimit("wallet-pay"), controller.RequestWaffoPay)
				selfRoute.POST("/waffo-pancake/amount", controller.RequestWaffoPancakeAmount)
				selfRoute.POST("/waffo-pancake/pay", middleware.UserCriticalRateLimit("wallet-pay"), controller.RequestWaffoPancakePay)
				selfRoute.POST("/aff_transfer", middleware.UserCriticalRateLimit("aff-transfer"), controller.TransferAffQuota)
				selfRoute.PUT("/setting", controller.UpdateUserSetting)

				// 2FA routes
				selfRoute.GET("/2fa/status", controller.Get2FAStatus)
				selfRoute.POST("/2fa/setup", middleware.UserCriticalRateLimit("security-verification"), middleware.DisableCache(), controller.Setup2FA)
				selfRoute.POST("/2fa/enable", middleware.UserCriticalRateLimit("security-verification"), middleware.DisableCache(), controller.Enable2FA)
				selfRoute.POST("/2fa/disable", middleware.DisableCache(), controller.Disable2FA)
				selfRoute.POST("/2fa/backup_codes", middleware.DisableCache(), controller.RegenerateBackupCodes)

				// Check-in routes
				selfRoute.GET("/checkin", controller.GetCheckinStatus)
				selfRoute.POST("/checkin", middleware.TurnstileCheck(), controller.DoCheckin)

				// Custom OAuth bindings
				selfRoute.GET("/oauth/bindings", controller.GetUserOAuthBindings)
				selfRoute.DELETE("/oauth/bindings/:provider_id", controller.UnbindCustomOAuth)
			}

			adminRoute := userRoute.Group("/")
			adminRoute.Use(middleware.AdminAuth())
			{
				for _, route := range userAdminPermissionRoutes {
					handlePermissionRoute(adminRoute, route.method, route.path, route.permission, route.handler)
				}
				// Custom authorization roles of a user (root only).
				adminRoute.GET("/:id/roles", middleware.RootAuth(), controller.GetUserAuthzRoles)
				adminRoute.PUT("/:id/roles", middleware.RootAuth(), middleware.DisableCache(), controller.SetUserAuthzRoles)
			}
		}

		// Subscription billing (plans, purchase, admin management)
		subscriptionRoute := apiRouter.Group("/subscription")
		subscriptionRoute.Use(middleware.UserAuth())
		{
			subscriptionRoute.GET("/plans", controller.GetSubscriptionPlans)
			subscriptionRoute.GET("/self", controller.GetSubscriptionSelf)
			subscriptionRoute.PUT("/self/preference", controller.UpdateSubscriptionPreference)
			subscriptionRoute.POST("/balance/pay", middleware.UserCriticalRateLimit("subscription-pay"), controller.SubscriptionRequestBalancePay)
			subscriptionRoute.POST("/epay/pay", middleware.UserCriticalRateLimit("subscription-pay"), controller.SubscriptionRequestEpay)
			subscriptionRoute.POST("/stripe/pay", middleware.UserCriticalRateLimit("subscription-pay"), controller.SubscriptionRequestStripePay)
			subscriptionRoute.POST("/creem/pay", middleware.UserCriticalRateLimit("subscription-pay"), controller.SubscriptionRequestCreemPay)
			subscriptionRoute.POST("/waffo-pancake/pay", middleware.UserCriticalRateLimit("subscription-pay"), controller.SubscriptionRequestWaffoPancakePay)
		}
		subscriptionAdminRoute := apiRouter.Group("/subscription/admin")
		subscriptionAdminRoute.Use(middleware.AdminAuth())
		{
			for _, route := range subscriptionAdminPermissionRoutes {
				handlePermissionRoute(subscriptionAdminRoute, route.method, route.path, route.permission, route.handler)
			}
		}

		// Subscription payment callbacks (no auth)
		apiRouter.POST("/subscription/epay/notify", anonymousRequestBodyLimit, controller.SubscriptionEpayNotify)
		apiRouter.GET("/subscription/epay/notify", controller.SubscriptionEpayNotify)
		apiRouter.GET("/subscription/epay/return", controller.SubscriptionEpayReturn)
		apiRouter.POST("/subscription/epay/return", anonymousRequestBodyLimit, controller.SubscriptionEpayReturn)
		optionRoute := apiRouter.Group("/option")
		optionRoute.Use(middleware.RootAuth())
		{
			optionRoute.GET("/", controller.GetOptions)
			optionRoute.GET("/request_policy", controller.GetRequestPolicy)
			optionRoute.PATCH("/request_policy", controller.UpdateRequestPolicy)
			optionRoute.PUT("/", controller.UpdateOption)
			optionRoute.PUT("/passkey/domains", controller.UpdatePasskeyDomains)
			optionRoute.GET("/model_pricing", controller.GetModelPricingConfig)
			optionRoute.PATCH("/model_pricing", controller.UpdateModelPricingConfig)
			optionRoute.POST("/model_pricing/convert", controller.PreviewModelPricingConversion)
			optionRoute.POST("/model_pricing/preview", controller.PreviewModelPricing)
			optionRoute.POST("/payment_compliance", controller.ConfirmPaymentCompliance)
			optionRoute.GET("/channel_affinity_cache", controller.GetChannelAffinityCacheStats)
			optionRoute.DELETE("/channel_affinity_cache", controller.ClearChannelAffinityCache)
			optionRoute.POST("/rest_model_ratio", controller.ResetModelRatio)
			optionRoute.GET("/waffo-pancake/catalog", controller.ListWaffoPancakeCatalog)
			optionRoute.POST("/waffo-pancake/pair", controller.CreateWaffoPancakePair)
			optionRoute.POST("/waffo-pancake/save", controller.SaveWaffoPancake)
			optionRoute.POST("/waffo-pancake/subscription-product", controller.CreateWaffoPancakeSubscriptionProduct)
			optionRoute.GET("/waffo-pancake/subscription-product-options", controller.ListWaffoPancakeSubscriptionProductOptions)
		}

		// Custom OAuth provider management (root only)
		customOAuthRoute := apiRouter.Group("/custom-oauth-provider")
		customOAuthRoute.Use(middleware.RootAuth())
		{
			customOAuthRoute.POST("/discovery", controller.FetchCustomOAuthDiscovery)
			customOAuthRoute.GET("/", controller.GetCustomOAuthProviders)
			customOAuthRoute.GET("/:id", controller.GetCustomOAuthProvider)
			customOAuthRoute.POST("/", controller.CreateCustomOAuthProvider)
			customOAuthRoute.PUT("/:id", controller.UpdateCustomOAuthProvider)
			customOAuthRoute.DELETE("/:id", controller.DeleteCustomOAuthProvider)
		}
		performanceRoute := apiRouter.Group("/performance")
		performanceRoute.Use(middleware.RootAuth())
		{
			performanceRoute.GET("/stats", controller.GetPerformanceStats)
			performanceRoute.DELETE("/disk_cache", controller.ClearDiskCache)
			performanceRoute.POST("/reset_stats", controller.ResetPerformanceStats)
			performanceRoute.POST("/gc", controller.ForceGC)
			performanceRoute.GET("/logs", controller.GetLogFiles)
			performanceRoute.DELETE("/logs", controller.CleanupLogFiles)
		}
		ratioSyncRoute := apiRouter.Group("/ratio_sync")
		ratioSyncRoute.Use(middleware.RootAuth())
		{
			ratioSyncRoute.GET("/channels", controller.GetSyncableChannels)
			ratioSyncRoute.POST("/fetch", controller.FetchUpstreamRatios)
		}
		taskPluginRoute := apiRouter.Group("/plugin/task")
		taskPluginRoute.Use(middleware.RootAuth())
		{
			taskPluginRoute.GET("", controller.ListTaskPlugins)
			taskPluginRoute.POST("", controller.UploadTaskPlugin)
			taskPluginRoute.PUT("", controller.UploadTaskPlugin)
			taskPluginRoute.GET("/runtime/status", controller.GetTaskPluginRuntime)
			taskPluginRoute.GET("/marketplace/sources", controller.GetTaskPluginMarketplaceSources)
			taskPluginRoute.PUT("/marketplace/sources", controller.UpdateTaskPluginMarketplaceSources)
			taskPluginRoute.GET("/:key", controller.GetTaskPlugin)
			taskPluginRoute.GET("/:key/icon", controller.GetTaskPluginIcon)
			taskPluginRoute.GET("/:key/versions", controller.GetTaskPluginVersions)
			taskPluginRoute.POST("/:key/activate", controller.ActivateTaskPlugin)
			taskPluginRoute.POST("/:key/status", controller.SetTaskPluginStatus)
			taskPluginRoute.POST("/:key/dryrun", controller.DryRunTaskPlugin)
			taskPluginRoute.DELETE("/:key/versions/:version", controller.DeleteTaskPluginVersion)
		}
		handlePermissionRoute(apiRouter.Group("", middleware.AdminAuth()), http.MethodGet, "/task_plugin_options", authz.TaskPluginBind, controller.GetTaskPluginOptions)
		registerChannelRoutes(apiRouter)
		registerAuthzRoutes(apiRouter)
		tokenRoute := apiRouter.Group("/token")
		tokenRoute.Use(middleware.UserAuth())
		tokenRoute.Use(middleware.TokenOperationAudit())
		{
			tokenRoute.GET("/", controller.GetAllTokens)
			tokenRoute.GET("/search", middleware.SearchRateLimit(), controller.SearchTokens)
			tokenRoute.GET("/auto-groups", controller.GetTokenAutoGroups)
			tokenRoute.GET("/:id", controller.GetToken)
			tokenRoute.POST("/:id/key", middleware.CriticalRateLimit(), middleware.DisableCache(), controller.GetTokenKey)
			tokenRoute.POST("/", controller.AddToken)
			tokenRoute.PUT("/", controller.UpdateToken)
			tokenRoute.DELETE("/:id", controller.DeleteToken)
			tokenRoute.POST("/batch", controller.DeleteTokenBatch)
			tokenRoute.POST("/batch/keys", middleware.CriticalRateLimit(), middleware.DisableCache(), controller.GetTokenKeysBatch)
		}

		usageRoute := apiRouter.Group("/usage")
		usageRoute.Use(middleware.CORS(), middleware.CriticalRateLimit())
		{
			tokenUsageRoute := usageRoute.Group("/token")
			tokenUsageRoute.Use(middleware.TokenAuthReadOnly())
			{
				tokenUsageRoute.GET("/", controller.GetTokenUsage)
			}
		}

		redemptionRoute := apiRouter.Group("/redemption")
		redemptionRoute.Use(middleware.AdminAuth())
		{
			handlePermissionRoute(redemptionRoute, http.MethodGet, "/", authz.RedemptionRead, controller.GetAllRedemptions)
			handlePermissionRoute(redemptionRoute, http.MethodGet, "/search", authz.RedemptionRead, controller.SearchRedemptions)
			handlePermissionRoute(redemptionRoute, http.MethodGet, "/:id", authz.RedemptionRead, controller.GetRedemption)
			handlePermissionRoute(redemptionRoute, http.MethodPost, "/", authz.RedemptionWrite, controller.AddRedemption)
			handlePermissionRoute(redemptionRoute, http.MethodPost, "/batch", authz.RedemptionWrite, controller.DeleteRedemptionBatch)
			handlePermissionRoute(redemptionRoute, http.MethodPut, "/", authz.RedemptionWrite, controller.UpdateRedemption)
			handlePermissionRoute(redemptionRoute, http.MethodDelete, "/invalid", authz.RedemptionWrite, controller.DeleteInvalidRedemption)
			handlePermissionRoute(redemptionRoute, http.MethodDelete, "/:id", authz.RedemptionWrite, controller.DeleteRedemption)
		}
		handlePermissionRoute(apiRouter.Group("", middleware.DisableCache(), middleware.AdminAuth()), http.MethodGet, "/audit", authz.AuditRead, controller.GetAuditLogs)
		apiRouter.GET("/audit/self", middleware.DisableCache(), middleware.UserAuth(), controller.GetAuditLogs)
		logRoute := apiRouter.Group("/log")
		logAdminRoute := logRoute.Group("", middleware.AdminAuth())
		handlePermissionRoute(logAdminRoute, http.MethodGet, "/", authz.LogRead, controller.GetAllLogs)
		handlePermissionRoute(logAdminRoute, http.MethodGet, "/stat", authz.LogRead, controller.GetLogsStat)
		logRoute.GET("/self/stat", middleware.UserAuth(), controller.GetLogsSelfStat)
		handlePermissionRoute(logAdminRoute, http.MethodGet, "/channel_affinity_usage_cache", authz.LogRead, controller.GetChannelAffinityUsageCacheStats)
		handlePermissionRoute(logAdminRoute, http.MethodGet, "/search", authz.LogRead, controller.SearchAllLogs)
		logRoute.GET("/self", middleware.UserAuth(), controller.GetUserLogs)
		logRoute.GET("/self/search", middleware.UserAuth(), middleware.SearchRateLimit(), controller.SearchUserLogs)

		systemTaskRoute := apiRouter.Group("/system-task")
		systemTaskRoute.Use(middleware.RootAuth())
		{
			systemTaskRoute.POST("/log-cleanup", controller.CreateLogCleanupSystemTask)
			systemTaskRoute.GET("/list", controller.ListSystemTasks)
			systemTaskRoute.DELETE("/history", controller.DeleteSystemTaskHistory)
			systemTaskRoute.GET("/current", controller.GetCurrentSystemTask)
			systemTaskRoute.GET("/:task_id", controller.GetSystemTask)
		}
		systemInfoRoute := apiRouter.Group("/system-info")
		systemInfoRoute.Use(middleware.RootAuth())
		{
			systemInfoRoute.GET("/instances", controller.ListSystemInstances)
			systemInfoRoute.DELETE("/stale-instances", controller.DeleteStaleSystemInstances)
			systemInfoRoute.DELETE("/instances/:node_name", controller.DeleteStaleSystemInstance)
		}

		dataRoute := apiRouter.Group("/data")
		dataAdminRoute := dataRoute.Group("", middleware.AdminAuth())
		handlePermissionRoute(dataAdminRoute, http.MethodGet, "/", authz.LogRead, controller.GetAllQuotaDates)
		handlePermissionRoute(dataAdminRoute, http.MethodGet, "/users", authz.LogRead, controller.GetQuotaDatesByUser)
		dataRoute.GET("/self", middleware.UserAuth(), controller.GetUserQuotaDates)
		handlePermissionRoute(dataAdminRoute, http.MethodGet, "/flow", authz.LogRead, controller.GetAllFlowQuotaDates)
		dataRoute.GET("/flow/self", middleware.UserAuth(), controller.GetUserFlowQuotaDates)

		logRoute.Use(middleware.CORS(), middleware.CriticalRateLimit())
		{
			logRoute.GET("/token", middleware.TokenAuthReadOnly(), controller.GetLogByKey)
		}
		groupRoute := apiRouter.Group("/group")
		groupRoute.Use(middleware.AdminAuth())
		{
			handlePermissionRoute(groupRoute, http.MethodGet, "/", authz.GroupRead, controller.GetGroups)
			handlePermissionRoute(groupRoute, http.MethodGet, "/usage", authz.GroupRead, controller.GetGroupUsage)
			handlePermissionRoute(groupRoute, http.MethodGet, "/ratios", authz.GroupRead, controller.GetGroupRatios)
			// Rewrites system options, so it needs the same access as settings.
			groupRoute.POST("/rename", middleware.RootAuth(), controller.RenameGroup)
		}

		prefillGroupRoute := apiRouter.Group("/prefill_group")
		prefillGroupRoute.Use(middleware.AdminAuth())
		{
			handlePermissionRoute(prefillGroupRoute, http.MethodGet, "/", authz.GroupRead, controller.GetPrefillGroups)
			handlePermissionRoute(prefillGroupRoute, http.MethodPost, "/", authz.GroupWrite, controller.CreatePrefillGroup)
			handlePermissionRoute(prefillGroupRoute, http.MethodPut, "/", authz.GroupWrite, controller.UpdatePrefillGroup)
			handlePermissionRoute(prefillGroupRoute, http.MethodDelete, "/:id", authz.GroupWrite, controller.DeletePrefillGroup)
		}

		mjRoute := apiRouter.Group("/mj")
		mjRoute.GET("/self", middleware.UserAuth(), controller.GetUserMidjourney)
		handlePermissionRoute(mjRoute.Group("", middleware.AdminAuth()), http.MethodGet, "/", authz.LogRead, controller.GetAllMidjourney)

		taskRoute := apiRouter.Group("/task")
		{
			taskRoute.GET("/self", middleware.UserAuth(), controller.GetUserTask)
			handlePermissionRoute(taskRoute.Group("", middleware.AdminAuth()), http.MethodGet, "", authz.LogRead, controller.GetAllTask)
			taskRoute.GET("/:task_id/artifacts", middleware.UserAuth(), controller.GetDashboardTaskArtifacts)
		}

		vendorRoute := apiRouter.Group("/vendors")
		vendorRoute.Use(middleware.AdminAuth())
		{
			handlePermissionRoute(vendorRoute, http.MethodPost, "/operations/preview", authz.ModelRead, controller.PreviewVendorOperation)
			handlePermissionRoute(vendorRoute, http.MethodPost, "/operations", authz.ModelWrite, controller.ApplyVendorOperation)
			handlePermissionRoute(vendorRoute, http.MethodGet, "/", authz.ModelRead, controller.GetAllVendors)
			handlePermissionRoute(vendorRoute, http.MethodGet, "/search", authz.ModelRead, controller.SearchVendors)
			handlePermissionRoute(vendorRoute, http.MethodGet, "/:id", authz.ModelRead, controller.GetVendorMeta)
			handlePermissionRoute(vendorRoute, http.MethodPost, "/", authz.ModelWrite, controller.CreateVendorMeta)
			handlePermissionRoute(vendorRoute, http.MethodPut, "/", authz.ModelWrite, controller.UpdateVendorMeta)
			handlePermissionRoute(vendorRoute, http.MethodDelete, "/:id", authz.ModelWrite, controller.DeleteVendorMeta)
		}

		modelsRoute := apiRouter.Group("/models")
		modelsRoute.Use(middleware.AdminAuth())
		{
			handlePermissionRoute(modelsRoute, http.MethodGet, "/sync_upstream/preview", authz.ModelRead, controller.SyncUpstreamPreview)
			handlePermissionRoute(modelsRoute, http.MethodPost, "/sync_upstream", authz.ModelWrite, controller.SyncUpstreamModels)
			handlePermissionRoute(modelsRoute, http.MethodPost, "/delete", authz.ModelWrite, controller.BatchDeleteModelMeta)
			handlePermissionRoute(modelsRoute, http.MethodGet, "/missing", authz.ModelRead, controller.GetMissingModels)
			handlePermissionRoute(modelsRoute, http.MethodGet, "/", authz.ModelRead, controller.GetAllModelsMeta)
			handlePermissionRoute(modelsRoute, http.MethodGet, "/search", authz.ModelRead, controller.SearchModelsMeta)
			handlePermissionRoute(modelsRoute, http.MethodGet, "/:id", authz.ModelRead, controller.GetModelMeta)
			handlePermissionRoute(modelsRoute, http.MethodPost, "/", authz.ModelWrite, controller.CreateModelMeta)
			handlePermissionRoute(modelsRoute, http.MethodPut, "/", authz.ModelWrite, controller.UpdateModelMeta)
			handlePermissionRoute(modelsRoute, http.MethodDelete, "/:id", authz.ModelWrite, controller.DeleteModelMeta)
		}

		// Deployments (model deployment management)
		deploymentsRoute := apiRouter.Group("/deployments")
		deploymentsRoute.Use(middleware.AdminAuth())
		{
			for _, route := range deploymentPermissionRoutes {
				handlePermissionRoute(deploymentsRoute, route.method, route.path, route.permission, route.handler)
			}
		}
	}
}

var userAdminPermissionRoutes = []permissionRoute{
	{method: http.MethodGet, path: "/", permission: authz.UserRead, handler: controller.GetAllUsers},
	{method: http.MethodGet, path: "/topup", permission: authz.BillingRead, handler: controller.GetAllTopUps},
	{method: http.MethodPost, path: "/topup/complete", permission: authz.BillingWrite, handler: controller.AdminCompleteTopUp},
	{method: http.MethodGet, path: "/search", permission: authz.UserRead, handler: controller.SearchUsers},
	{method: http.MethodGet, path: "/:id/oauth/bindings", permission: authz.UserRead, handler: controller.GetUserOAuthBindingsByAdmin},
	{method: http.MethodDelete, path: "/:id/oauth/bindings/:provider_id", permission: authz.UserWrite, handler: controller.UnbindCustomOAuthByAdmin},
	{method: http.MethodDelete, path: "/:id/bindings/:binding_type", permission: authz.UserWrite, handler: controller.AdminClearUserBinding},
	{method: http.MethodGet, path: "/:id", permission: authz.UserRead, handler: controller.GetUser},
	{method: http.MethodPost, path: "/", permission: authz.UserWrite, handler: controller.CreateUser},
	{method: http.MethodPost, path: "/manage", permission: authz.UserWrite, handler: controller.ManageUser},
	{method: http.MethodPut, path: "/", permission: authz.UserWrite, handler: controller.UpdateUser},
	{method: http.MethodDelete, path: "/:id", permission: authz.UserWrite, handler: controller.DeleteUser},
	{method: http.MethodDelete, path: "/:id/reset_passkey", permission: authz.UserWrite, handler: controller.AdminResetPasskey},
	{method: http.MethodGet, path: "/2fa/stats", permission: authz.UserRead, handler: controller.Admin2FAStats},
	{method: http.MethodDelete, path: "/:id/2fa", permission: authz.UserWrite, handler: controller.AdminDisable2FA},
}

var subscriptionAdminPermissionRoutes = []permissionRoute{
	{method: http.MethodGet, path: "/plans", permission: authz.BillingRead, handler: controller.AdminListSubscriptionPlans},
	{method: http.MethodPost, path: "/plans", permission: authz.BillingWrite, handler: controller.AdminCreateSubscriptionPlan},
	{method: http.MethodPut, path: "/plans/:id", permission: authz.BillingWrite, handler: controller.AdminUpdateSubscriptionPlan},
	{method: http.MethodPatch, path: "/plans/:id", permission: authz.BillingWrite, handler: controller.AdminUpdateSubscriptionPlanStatus},
	{method: http.MethodPost, path: "/bind", permission: authz.BillingWrite, handler: controller.AdminBindSubscription},
	{method: http.MethodPost, path: "/plans/:id/subscriptions/reset", permission: authz.BillingWrite, handler: controller.AdminResetPlanSubscriptions},
	{method: http.MethodGet, path: "/users/:id/subscriptions", permission: authz.BillingRead, handler: controller.AdminListUserSubscriptions},
	{method: http.MethodPost, path: "/users/:id/subscriptions", permission: authz.BillingWrite, handler: controller.AdminCreateUserSubscription},
	{method: http.MethodPost, path: "/users/:id/subscriptions/reset", permission: authz.BillingWrite, handler: controller.AdminResetUserSubscriptionsByPlan},
	{method: http.MethodPost, path: "/user_subscriptions/:id/invalidate", permission: authz.BillingWrite, handler: controller.AdminInvalidateUserSubscription},
	{method: http.MethodDelete, path: "/user_subscriptions/:id", permission: authz.BillingWrite, handler: controller.AdminDeleteUserSubscription},
}

var deploymentPermissionRoutes = []permissionRoute{
	{method: http.MethodGet, path: "/settings", permission: authz.DeploymentRead, handler: controller.GetModelDeploymentSettings},
	{method: http.MethodPost, path: "/settings/test-connection", permission: authz.DeploymentWrite, handler: controller.TestIoNetConnection},
	{method: http.MethodGet, path: "/", permission: authz.DeploymentRead, handler: controller.GetAllDeployments},
	{method: http.MethodGet, path: "/search", permission: authz.DeploymentRead, handler: controller.SearchDeployments},
	{method: http.MethodPost, path: "/test-connection", permission: authz.DeploymentWrite, handler: controller.TestIoNetConnection},
	{method: http.MethodGet, path: "/hardware-types", permission: authz.DeploymentRead, handler: controller.GetHardwareTypes},
	{method: http.MethodGet, path: "/locations", permission: authz.DeploymentRead, handler: controller.GetLocations},
	{method: http.MethodGet, path: "/available-replicas", permission: authz.DeploymentRead, handler: controller.GetAvailableReplicas},
	{method: http.MethodPost, path: "/price-estimation", permission: authz.DeploymentRead, handler: controller.GetPriceEstimation},
	{method: http.MethodGet, path: "/check-name", permission: authz.DeploymentRead, handler: controller.CheckClusterNameAvailability},
	{method: http.MethodPost, path: "/", permission: authz.DeploymentWrite, handler: controller.CreateDeployment},
	{method: http.MethodGet, path: "/:id", permission: authz.DeploymentRead, handler: controller.GetDeployment},
	{method: http.MethodGet, path: "/:id/logs", permission: authz.DeploymentRead, handler: controller.GetDeploymentLogs},
	{method: http.MethodGet, path: "/:id/containers", permission: authz.DeploymentRead, handler: controller.ListDeploymentContainers},
	{method: http.MethodGet, path: "/:id/containers/:container_id", permission: authz.DeploymentRead, handler: controller.GetContainerDetails},
	{method: http.MethodPut, path: "/:id", permission: authz.DeploymentWrite, handler: controller.UpdateDeployment},
	{method: http.MethodPut, path: "/:id/name", permission: authz.DeploymentWrite, handler: controller.UpdateDeploymentName},
	{method: http.MethodPost, path: "/:id/extend", permission: authz.DeploymentWrite, handler: controller.ExtendDeployment},
	{method: http.MethodDelete, path: "/:id", permission: authz.DeploymentWrite, handler: controller.DeleteDeployment},
}
