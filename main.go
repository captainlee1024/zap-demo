// Package main provides ...
package main

import (
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/natefinch/lumberjack"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// 声明全局变量
var (
	// 声明全局的 logger 变量
	logger *zap.Logger

	// 声明全局的 sugarLogger 变量
	sugarLogger *zap.SugaredLogger
)

// gin 中使用 zap
func main() {
	//r := gin.Default() // 默认使用 Logger() 和 Recorvery() 两个中间件

	InitLogger()
	r := gin.New()                                      // 新建一个没有中间件的路由引擎
	r.Use(GinLogger(logger), GinRecovery(logger, true)) // 注册我们自己实现的结合 zap 的中间件
	r.GET("/hello", func(c *gin.Context) {
		c.String(http.StatusOK, "hello gin zap")
	})
	r.Run()
}

// GinLogger 接收gin框架默认的日志
func GinLogger(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery
		c.Next()

		cost := time.Since(start)
		logger.Info(path,
			zap.Int("status", c.Writer.Status()),
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.String("query", query),
			zap.String("ip", c.ClientIP()),
			zap.String("user-agent", c.Request.UserAgent()),
			zap.String("errors", c.Errors.ByType(gin.ErrorTypePrivate).String()),
			zap.Duration("cost", cost),
		)
	}
}

// GinRecovery recover掉项目可能出现的panic
func GinRecovery(logger *zap.Logger, stack bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// Check for a broken connection, as it is not really a
				// condition that warrants a panic stack trace.
				var brokenPipe bool
				if ne, ok := err.(*net.OpError); ok {
					if se, ok := ne.Err.(*os.SyscallError); ok {
						if strings.Contains(strings.ToLower(se.Error()), "broken pipe") || strings.Contains(strings.ToLower(se.Error()), "connection reset by peer") {
							brokenPipe = true
						}
					}
				}

				httpRequest, _ := httputil.DumpRequest(c.Request, false)
				if brokenPipe {
					logger.Error(c.Request.URL.Path,
						zap.Any("error", err),
						zap.String("request", string(httpRequest)),
					)
					// If the connection is dead, we can't write a status to it.
					c.Error(err.(error)) // nolint: errcheck
					c.Abort()
					return
				}

				if stack {
					logger.Error("[Recovery from panic]",
						zap.Any("error", err),
						zap.String("request", string(httpRequest)),
						zap.String("stack", string(debug.Stack())),
					)
				} else {
					logger.Error("[Recovery from panic]",
						zap.Any("error", err),
						zap.String("request", string(httpRequest)),
					)
				}
				c.AbortWithStatus(http.StatusInternalServerError)
			}
		}()
		c.Next()
	}
}

//func main() {
//	InitLogger()
//	defer logger.Sync() // 在退出或出现错误的时候刷新缓存里的日志信息
//	sugarExample("www.google.com")
//	loggerExample("www.google.com")
//	sugarExample("http://www.baidu.com")
//	loggerExample("http://www.baidu.com")

// 分割归档测试
//	for i := 0; i < 10000; i++ {
//		logger.Info("test for log rotate...")
//		sugarLogger.Info("sugar test for log rotate...")
//	}
//}

// 初始化全局变量 logger sugarLogger
func InitLogger() {
	writerSyncer := getLogWriter()
	encoder := getEncoder()
	core := zapcore.NewCore(encoder, writerSyncer, zapcore.DebugLevel)

	//logger = zap.New(core)
	// 添加 Option zap.AddCaller() 函数调用信息
	logger = zap.New(core, zap.AddCaller())
	sugarLogger = logger.Sugar()

	//var err error
	//logger, err = zap.NewProduction() // 生产环境
	//logger, err = zap.NewDevelopment() // 开发环境
	//logger = zap.NewExample()
	//if err != nil {
	//	fmt.Printf("initlogger failed, err:%v\n", err)
	//}

	//sugarLogger = logger.Sugar()
}

func getEncoder() zapcore.Encoder {
	// 配置
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:       "ts",
		LevelKey:      "level",
		NameKey:       "logger", // 名字是什么
		CallerKey:     "caller", // 调用者的名字
		MessageKey:    "msg",
		StacktraceKey: "stacktrace",
		LineEnding:    zapcore.DefaultLineEnding,
		EncodeLevel:   zapcore.LowercaseLevelEncoder,
		//EncodeTime: zapcore.EpochTimeEncoder, // 默认的时间编码器
		EncodeTime:     zapcore.ISO8601TimeEncoder, // 修改之后的时间编码器
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	// 配置 JSON 编码器
	//return zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig())

	// 配置 Console 编码器
	return zapcore.NewConsoleEncoder(encoderConfig)
}

/*
func getLogWriter() zapcore.WriteSyncer {
	//file, _ := os.Create("./test.log")
	// 修改为追加的方式添加日志
	file, _ := os.OpenFile("./test.log", os.O_CREATE|os.O_APPEND|os.O_RDWR, 0744)
	return zapcore.AddSync(file)
}
*/

// 配置使用第三方日志分割归档 Lumberjack
func getLogWriter() zapcore.WriteSyncer {
	lumberJackLogger := &lumberjack.Logger{
		Filename:   "./test.log", // 文件名称
		MaxSize:    1,            // 单个文件最大 10 单位M
		MaxBackups: 5,            // 最大备份数量
		MaxAge:     30,           // 最大备份天数（最多保存多少天的备份）
		Compress:   false,        // 是否压缩（日志量比较大，或者磁盘空间有限可以开启，默认关闭）
	}
	return zapcore.AddSync(lumberJackLogger)
}

// SugarLogger示例
func sugarExample(url string) {
	//defer logger.Sync() // 在退出或出现错误的时候刷新缓存里的日志信息
	//sugar := logger.Sugar()

	sugarLogger.Debugf("Trying to hit GET request fo %s", url)
	resp, err := http.Get(url)
	if err != nil { // 如果连接失败，打印一条 Error 级别的日志
		sugarLogger.Errorf("Error fetching URL %s:Error=%s", url, err)

		//sugarLogger.Error(
		//	"Error fetching url...",
		//	"url", url,
		//	"err", err,
		//)
	} else { // 如果连接成功，打印一条 info 级别的日志
		sugarLogger.Infof("Success! statusCode = %sfor URL %s", resp.Status, url)

		/*sugarLogger.Info(
			"Success...",
			"statusCode", resp.Status,
			"url", url,
		)
		*/
		resp.Body.Close()
	}
}

// logger示例
func loggerExample(url string) {
	resp, err := http.Get(url)
	if err != nil {
		logger.Error(
			"Error fetching url...",
			zap.String("url", url),
			zap.Error(err),
		)
	} else {
		logger.Info(
			"Success...",
			zap.String("statusCode", resp.Status),
			zap.String("url", url),
		)
		resp.Body.Close()
	}
}
