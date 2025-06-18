package echocore

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"database/sql"
	"encoding/hex"
	"errors"
	"github.com/caarlos0/env/v11"
	"github.com/go-sql-driver/mysql"
	"github.com/gorilla/sessions"
	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/mrccnt/echocore/redstore"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"io/fs"
	"net/http"
	"os"
	"os/signal"
	"reflect"
	"sync"
	"syscall"
	"time"
)

type Core struct {
	Config    *Config
	Gorm      *gorm.DB
	Redis     *redis.Client
	SessStore *redstore.RedisStore
	TmpDir    string
}

type InitHandler func() error

func init() {
	confLogger(logrus.StandardLogger())
}

func NewCore() (*Core, error) {

	var err error

	if _, err = os.Stat(".env"); err == nil {
		if err = godotenv.Load(); err != nil {
			return nil, err
		}
	}

	core := &Core{Config: new(Config)}
	if err = env.Parse(core.Config); err != nil {
		return nil, err
	}

	// if core.Config.IsDebug() {
	//	var bs []byte
	//	if bs, err = json.MarshalIndent(core.Config, "", "  "); err != nil {
	//		return nil, err
	//	} else {
	//		fmt.Println(string(bs))
	//	}
	//}

	if err = NewValidator().Validate(core.Config); err != nil {
		return nil, err
	}

	logrus.SetLevel(core.Config.LogrusLevel())

	return core, nil
}

func NewEcho(core *Core, pre ...echo.MiddlewareFunc) *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.Logger.SetLevel(core.Config.GommonLevel())
	e.Validator = NewValidator()
	e.Pre(pre...)
	e.Use(middleware.Recover())
	e.Use(middleware.Secure())
	e.Use(middleware.RemoveTrailingSlash())
	e.Use(middleware.RequestID())
	e.Use(ServerHeaderMiddleware(core.Config.App.ServerHeader))
	e.Use(GzipMiddleware(core.Config.App.GzipCompr))
	e.Use(ContextMiddleware(CtxCore, core))
	return e
}

func Run(core *Core, e *echo.Echo) {

	chsig := make(chan os.Signal, 1)
	signal.Notify(chsig, os.Interrupt, syscall.SIGTERM)

	var wg sync.WaitGroup
	wg.Add(1)
	go core.ListenSig(chsig, e, &wg)

	if err := e.Start(core.Config.App.Bind); err != nil {
		if errors.Is(err, http.ErrServerClosed) {
			logDown(e, err.Error())
		} else {
			logDownErr(e, err.Error())
		}
	}

	wg.Wait()
}

func (c *Core) Init(inits []InitHandler) error {
	for _, init := range inits {
		if err := init(); err != nil {
			return err
		}
	}
	return nil
}

func (c *Core) InitGorm() InitHandler {
	return func() error {
		logInit("Gorm")

		const (
			proto     = "tcp"
			pCharset  = "charset"
			pTLS      = "tls"
			tlsKeyLen = 12
		)

		loc, err := time.LoadLocation(c.Config.DB.Timezone)
		if err != nil {
			return err
		}

		mycfg := mysql.NewConfig()
		mycfg.User = c.Config.DB.User
		mycfg.Passwd = c.Config.DB.Pass
		mycfg.DBName = c.Config.DB.Name
		mycfg.Net = proto
		mycfg.Addr = c.Config.DB.Addr
		mycfg.Collation = c.Config.DB.Collation
		mycfg.Params = map[string]string{pCharset: c.Config.DB.Charset}
		mycfg.Loc = loc
		mycfg.MultiStatements = c.Config.DB.Multi
		mycfg.ParseTime = c.Config.DB.ParseTime

		if c.Config.IsTLSConfiguredDB() {
			var tlsCfg *tls.Config
			if tlsCfg, err = c.Config.TLSConfigDB(); err != nil {
				return err
			}
			bs := make([]byte, tlsKeyLen)
			if _, err = rand.Read(bs); err != nil {
				return err
			}
			tlsKey := hex.EncodeToString(bs)
			if err = mysql.RegisterTLSConfig(tlsKey, tlsCfg); err != nil {
				return err
			}
			mycfg.Params[pTLS] = tlsKey
		}

		glog := logrus.New()
		confLogger(glog)
		glog.SetLevel(logrus.ErrorLevel)

		if c.Gorm, err = gorm.Open(gormmysql.Open(mycfg.FormatDSN()), &gorm.Config{
			Logger: logger.New(glog, logger.Config{LogLevel: logger.Error}),
		}); err != nil {
			return err
		}

		var db *sql.DB
		if db, err = c.Gorm.DB(); err != nil {
			return err
		}

		db.SetMaxIdleConns(c.Config.DB.MaxIdle)
		db.SetMaxOpenConns(c.Config.DB.MaxOpen)
		db.SetConnMaxLifetime(time.Second * time.Duration(c.Config.DB.MaxLife))

		return db.Ping()
	}
}

func (c *Core) InitRedis() InitHandler {
	return func() error {
		logInit("Redis")
		c.Redis = redis.NewClient(&redis.Options{
			Addr:     c.Config.Redis.Addr,
			Username: c.Config.Redis.User,
			Password: c.Config.Redis.Pass,
		})
		if err := c.Redis.Ping(context.Background()).Err(); err != nil {
			_ = c.Redis.Close()
			return err
		}
		return nil
	}
}

func (c *Core) InitSessStore() InitHandler {
	return func() error {
		logInit("Session")

		var err error
		c.SessStore, err = redstore.NewRedisStore(context.Background(), c.Redis)
		if err != nil {
			return err
		}
		c.SessStore.KeyPrefix("session:")
		c.SessStore.Options(sessions.Options{
			Path:     c.Config.Session.Path,
			Domain:   c.Config.Session.Domain,
			MaxAge:   c.Config.Session.MaxAge,
			Secure:   c.Config.Session.Secure,
			HttpOnly: c.Config.Session.HTTPOnly,
			SameSite: c.Config.Session.SameSite,
		})
		return nil
	}
}

func (c *Core) InitTmpDir() InitHandler {
	return func() error {
		logInit("TmpDir")
		var err error
		c.TmpDir, err = os.MkdirTemp("", "")
		return err
	}
}

func (c *Core) InitCopyFs(dir string, fsys fs.FS) InitHandler {
	return func() error {
		logInit("CopyFS")
		return os.CopyFS(dir, fsys)
	}
}

func (c *Core) ListenSig(ch chan os.Signal, e *echo.Echo, wg *sync.WaitGroup) {

	sig := <-ch
	logDown(ch, "Received: "+sig.String())
	logDown(e, "Shutting down")

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(c.Config.App.EchoTimeout)*time.Second)
	defer cancel()
	if err := e.Shutdown(ctx); err != nil {
		logDown(e, err.Error())
	}

	c.Shutdown()

	wg.Done()
}

func (c *Core) Shutdown() {

	if c.SessStore != nil {
		logDown(c.SessStore, "Close")
		_ = c.SessStore.Close()
		c.SessStore = nil
	}

	if c.Redis != nil {
		logDown(c.Redis, "Close")
		_ = c.Redis.Close()
		c.Redis = nil
	}

	if c.Gorm != nil {
		logDown(c.Gorm, "Close")
		if db, err := c.Gorm.DB(); err == nil {
			_ = db.Close()
		}
		c.Gorm = nil
	}

	if c.TmpDir != "" {
		logDown(nil, "TmpDir Cleanup")
		_ = os.RemoveAll(c.TmpDir)
		c.TmpDir = ""
	}
}

func logInit(msg string) {
	logrus.Debugf("[Init] %s", msg)
}

func logDown(obj any, msg string) {
	logrus.Debugf("[Shutdown] [%s] %s", reflect.TypeOf(obj).String(), msg)
}

func logDownErr(obj any, msg string) {
	logrus.Errorf("[Shutdown] [%s] %s", reflect.TypeOf(obj).String(), msg)
}

func confLogger(l *logrus.Logger) {
	l.SetOutput(os.Stdout)
	l.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: time.RFC3339,
		PadLevelText:    true,
	})
}
