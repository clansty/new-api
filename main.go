package main

import (
	"bytes"
	"context"
	"embed"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/oauth"
	"github.com/QuantumNous/new-api/relay"
	"github.com/QuantumNous/new-api/router"
	"github.com/QuantumNous/new-api/service"
	_ "github.com/QuantumNous/new-api/setting/performance_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	_ "net/http/pprof"
)

//go:embed web/dist
var buildFS embed.FS

//go:embed web/dist/index.html
var indexPage []byte

func main() {
	startTime := time.Now()

	err := InitResources()
	if err != nil {
		common.FatalLog("failed to initialize resources: " + err.Error())
		return
	}

	common.SysLog("New API " + common.Version + " started")
	if os.Getenv("GIN_MODE") != "debug" {
		gin.SetMode(gin.ReleaseMode)
	}
	if common.DebugEnabled {
		common.SysLog("running in debug mode")
	}

	if common.RedisEnabled {
		// for compatibility with old versions
		common.MemoryCacheEnabled = true
	}
	if common.MemoryCacheEnabled {
		common.SysLog("memory cache enabled")
		common.SysLog(fmt.Sprintf("sync frequency: %d seconds", common.SyncFrequency))

		// Add panic recovery and retry for InitChannelCache
		func() {
			defer func() {
				if r := recover(); r != nil {
					common.SysLog(fmt.Sprintf("InitChannelCache panic: %v, retrying once", r))
					// Retry once
					_, _, fixErr := model.FixAbility()
					if fixErr != nil {
						common.FatalLog(fmt.Sprintf("InitChannelCache failed: %s", fixErr.Error()))
					}
				}
			}()
			model.InitChannelCache()
		}()

		go model.SyncChannelCache(common.SyncFrequency)
	}

	// 热更新配置
	go model.SyncOptions(common.SyncFrequency)

	// 数据看板
	go model.UpdateQuotaData()

	if os.Getenv("CHANNEL_UPDATE_FREQUENCY") != "" {
		frequency, err := strconv.Atoi(os.Getenv("CHANNEL_UPDATE_FREQUENCY"))
		if err != nil {
			common.FatalLog("failed to parse CHANNEL_UPDATE_FREQUENCY: " + err.Error())
		}
		go controller.AutomaticallyUpdateChannels(frequency)
	}

	go controller.AutomaticallyTestChannels()

	// Codex credential auto-refresh check every 10 minutes, refresh when expires within 1 day
	service.StartCodexCredentialAutoRefreshTask()

	// Subscription quota reset task (daily/weekly/monthly/custom)
	service.StartSubscriptionQuotaResetTask()

	// Wire task polling adaptor factory (breaks service -> relay import cycle)
	service.GetTaskAdaptorFunc = func(platform constant.TaskPlatform) service.TaskPollingAdaptor {
		a := relay.GetTaskAdaptor(platform)
		if a == nil {
			return nil
		}
		return a
	}

	// Channel upstream model update check task
	controller.StartChannelUpstreamModelUpdateTask()

	if common.IsMasterNode && constant.UpdateTask {
		gopool.Go(func() {
			controller.UpdateMidjourneyTaskBulk()
		})
		gopool.Go(func() {
			controller.UpdateTaskBulk()
		})
	}
	if os.Getenv("BATCH_UPDATE_ENABLED") == "true" {
		common.BatchUpdateEnabled = true
		common.SysLog("batch update enabled with interval " + strconv.Itoa(common.BatchUpdateInterval) + "s")
		model.InitBatchUpdater()
	}

	if os.Getenv("ENABLE_PPROF") == "true" {
		gopool.Go(func() {
			log.Println(http.ListenAndServe("0.0.0.0:8005", nil))
		})
		go common.Monitor()
		common.SysLog("pprof enabled")
	}

	err = common.StartPyroScope()
	if err != nil {
		common.SysError(fmt.Sprintf("start pyroscope error : %v", err))
	}

	// Initialize HTTP server
	server := gin.New()
	server.Use(gin.CustomRecovery(func(c *gin.Context, err any) {
		common.SysLog(fmt.Sprintf("panic detected: %v", err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"message": fmt.Sprintf("Panic detected, error: %v. Please submit a issue here: https://github.com/Calcium-Ion/new-api", err),
				"type":    "new_api_panic",
			},
		})
	}))
	// This will cause SSE not to work!!!
	//server.Use(gzip.Gzip(gzip.DefaultCompression))
	server.Use(middleware.RequestId())
	server.Use(middleware.PoweredBy())
	server.Use(middleware.I18n())
	middleware.SetUpLogger(server)
	// Initialize session store
	store := cookie.NewStore([]byte(common.SessionSecret))
	store.Options(sessions.Options{
		Path:     "/",
		MaxAge:   2592000, // 30 days
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteStrictMode,
	})
	server.Use(sessions.Sessions("session", store))

	InjectUmamiAnalytics()
	InjectGoogleAnalytics()

	// 设置路由
	router.SetRouter(server, buildFS, indexPage)
	var port = os.Getenv("PORT")
	if port == "" {
		port = strconv.Itoa(*common.Port)
	}

	// Log startup success message
	common.LogStartupSuccess(startTime, port)

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: server,
	}

	serverErrCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrCh <- err
		}
		close(serverErrCh)
	}()

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErrCh:
		if err != nil {
			common.FatalLog("failed to start HTTP server: " + err.Error())
		}
		return
	case sig := <-signalCh:
		common.SysLog(fmt.Sprintf("shutdown signal received: %s", sig))
	}

	gracefulShutdown(srv)
}

// gracefulShutdown 协调优雅关闭序列, 顺序很重要:
//  1. 翻转 IsShuttingDown 标志并取消 ShutdownCtx, 让 /api/status 立刻返回 503,
//     同时通知所有后台 goroutine 退出循环;
//  2. 给 Docker Swarm / 上游 LB 一段缓冲时间观察到 health 失败并停止派发新流量;
//  3. http.Server.Shutdown 等待所有进行中的 HTTP / SSE 请求自然完成(SSE 通过
//     Flusher 而非 hijack, Shutdown 会等它们); 若超时则 srv.Close 强制断,
//     并额外留一小段时间让 handler 派生的同步副作用收尾;
//  4. WaitBackground 等 GoTracked 派生的关键 fire-and-forget goroutine 完成;
//  5. 停 BatchUpdater (它用独立 ctx, 在此前持续 tick 写库, 缩短崩溃丢数据窗口);
//  6. 带 deadline 重试 flush 直到所有内存中的 batch / quota 数据落库;
//  7. 关闭 Redis 与 DB 连接.
func gracefulShutdown(srv *http.Server) {
	common.SysLog("graceful shutdown: marking unhealthy and cancelling background tasks")
	common.TriggerShutdown()

	drainDelay := time.Duration(common.GetEnvOrDefault("SHUTDOWN_DRAIN_DELAY_SECONDS", 5)) * time.Second
	if drainDelay > 0 {
		common.SysLog(fmt.Sprintf("graceful shutdown: waiting %s for upstream LB to remove this instance", drainDelay))
		time.Sleep(drainDelay)
	}

	httpTimeout := time.Duration(common.GetEnvOrDefault("SHUTDOWN_HTTP_TIMEOUT_SECONDS", 14*60)) * time.Second
	common.SysLog(fmt.Sprintf("graceful shutdown: waiting for in-flight HTTP requests (timeout %s)", httpTimeout))
	shutdownCtx, cancel := context.WithTimeout(context.Background(), httpTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		common.SysError("graceful shutdown: http server shutdown error (forcing close): " + err.Error())
		// Shutdown 超时, 强制关闭剩余连接并等待 handler 派生的同步副作用 (退款/日志写入)
		// 在 BatchUpdater 还活着的时候完成入队. 30s 是经验值, 足够 Refund / RecordConsumeLog
		// 这些同步路径跑完.
		_ = srv.Close()
		postCloseGrace := time.Duration(common.GetEnvOrDefault("SHUTDOWN_POST_CLOSE_GRACE_SECONDS", 30)) * time.Second
		common.SysLog(fmt.Sprintf("graceful shutdown: waiting %s for handler side-effects after force close", postCloseGrace))
		time.Sleep(postCloseGrace)
	}

	// 等待 GoTracked 派生的后台任务 (如 testAllChannels) 完成, 它们可能写 batch / DB.
	bgTimeout := time.Duration(common.GetEnvOrDefault("SHUTDOWN_BACKGROUND_TIMEOUT_SECONDS", 60)) * time.Second
	common.SysLog(fmt.Sprintf("graceful shutdown: waiting for tracked background tasks (timeout %s)", bgTimeout))
	common.WaitBackground(bgTimeout)

	common.SysLog("graceful shutdown: stopping batch updater")
	model.StopBatchUpdater()

	flushDeadline := time.Now().Add(time.Duration(common.GetEnvOrDefault("SHUTDOWN_FLUSH_TIMEOUT_SECONDS", 30)) * time.Second)
	flushWithRetry(flushDeadline)

	if common.RedisEnabled {
		if err := common.CloseRedisClient(); err != nil {
			common.SysError("graceful shutdown: redis close error: " + err.Error())
		}
	}

	if err := model.CloseDB(); err != nil {
		common.SysError("graceful shutdown: db close error: " + err.Error())
	}

	common.SysLog("graceful shutdown: done")
}

// flushWithRetry 在 deadline 内反复尝试 flush batch / quota cache,
// 退避 1s/2s/4s/8s. 任一调用返回 nil 即结束当前 flush;
// 两者都 nil 时整体提前退出, 避免无谓等待.
func flushWithRetry(deadline time.Time) {
	common.SysLog("graceful shutdown: flushing in-memory batch updates")
	backoff := 1 * time.Second
	for {
		batchErr := model.FlushBatchUpdater()
		quotaErr := model.SaveQuotaDataCache()
		if batchErr == nil && quotaErr == nil {
			common.SysLog("graceful shutdown: flush complete")
			return
		}
		if time.Now().After(deadline) {
			common.SysError(fmt.Sprintf("graceful shutdown: flush deadline reached with pending data (batch_err=%v quota_err=%v)", batchErr, quotaErr))
			return
		}
		common.SysError(fmt.Sprintf("graceful shutdown: flush failed, retrying in %s (batch_err=%v quota_err=%v)", backoff, batchErr, quotaErr))
		time.Sleep(backoff)
		if backoff < 8*time.Second {
			backoff *= 2
		}
	}
}

func InjectUmamiAnalytics() {
	analyticsInjectBuilder := &strings.Builder{}
	if os.Getenv("UMAMI_WEBSITE_ID") != "" {
		umamiSiteID := os.Getenv("UMAMI_WEBSITE_ID")
		umamiScriptURL := os.Getenv("UMAMI_SCRIPT_URL")
		if umamiScriptURL == "" {
			umamiScriptURL = "https://analytics.umami.is/script.js"
		}
		analyticsInjectBuilder.WriteString("<script defer src=\"")
		analyticsInjectBuilder.WriteString(umamiScriptURL)
		analyticsInjectBuilder.WriteString("\" data-website-id=\"")
		analyticsInjectBuilder.WriteString(umamiSiteID)
		analyticsInjectBuilder.WriteString("\"></script>")
	}
	analyticsInjectBuilder.WriteString("<!--Umami QuantumNous-->\n")
	analyticsInject := analyticsInjectBuilder.String()
	indexPage = bytes.ReplaceAll(indexPage, []byte("<!--umami-->\n"), []byte(analyticsInject))
}

func InjectGoogleAnalytics() {
	analyticsInjectBuilder := &strings.Builder{}
	if os.Getenv("GOOGLE_ANALYTICS_ID") != "" {
		gaID := os.Getenv("GOOGLE_ANALYTICS_ID")
		// Google Analytics 4 (gtag.js)
		analyticsInjectBuilder.WriteString("<script async src=\"https://www.googletagmanager.com/gtag/js?id=")
		analyticsInjectBuilder.WriteString(gaID)
		analyticsInjectBuilder.WriteString("\"></script>")
		analyticsInjectBuilder.WriteString("<script>")
		analyticsInjectBuilder.WriteString("window.dataLayer = window.dataLayer || [];")
		analyticsInjectBuilder.WriteString("function gtag(){dataLayer.push(arguments);}")
		analyticsInjectBuilder.WriteString("gtag('js', new Date());")
		analyticsInjectBuilder.WriteString("gtag('config', '")
		analyticsInjectBuilder.WriteString(gaID)
		analyticsInjectBuilder.WriteString("');")
		analyticsInjectBuilder.WriteString("</script>")
	}
	analyticsInjectBuilder.WriteString("<!--Google Analytics QuantumNous-->\n")
	analyticsInject := analyticsInjectBuilder.String()
	indexPage = bytes.ReplaceAll(indexPage, []byte("<!--Google Analytics-->\n"), []byte(analyticsInject))
}

func InitResources() error {
	// Initialize resources here if needed
	// This is a placeholder function for future resource initialization
	err := godotenv.Load(".env")
	if err != nil {
		if common.DebugEnabled {
			common.SysLog("No .env file found, using default environment variables. If needed, please create a .env file and set the relevant variables.")
		}
	}

	// 加载环境变量
	common.InitEnv()

	logger.SetupLogger()

	// Initialize model settings
	ratio_setting.InitRatioSettings()

	service.InitHttpClient()

	service.InitTokenEncoders()

	// Initialize SQL Database
	err = model.InitDB()
	if err != nil {
		common.FatalLog("failed to initialize database: " + err.Error())
		return err
	}

	model.CheckSetup()

	// Initialize options, should after model.InitDB()
	model.InitOptionMap()

	// 清理旧的磁盘缓存文件
	common.CleanupOldCacheFiles()

	// 初始化模型
	model.GetPricing()

	// Initialize SQL Database
	err = model.InitLogDB()
	if err != nil {
		return err
	}

	// Initialize Redis
	err = common.InitRedisClient()
	if err != nil {
		return err
	}

	// 启动系统监控
	common.StartSystemMonitor()

	// Initialize i18n
	err = i18n.Init()
	if err != nil {
		common.SysError("failed to initialize i18n: " + err.Error())
		// Don't return error, i18n is not critical
	} else {
		common.SysLog("i18n initialized with languages: " + strings.Join(i18n.SupportedLanguages(), ", "))
	}
	// Register user language loader for lazy loading
	i18n.SetUserLangLoader(model.GetUserLanguage)

	// Load custom OAuth providers from database
	err = oauth.LoadCustomProviders()
	if err != nil {
		common.SysError("failed to load custom OAuth providers: " + err.Error())
		// Don't return error, custom OAuth is not critical
	}

	return nil
}
