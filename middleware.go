package echocore

import (
	"embed"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/mrccnt/echocore/redstore"
	"github.com/sirupsen/logrus"
	"net/http"
	"slices"
	"time"
)

func LoggerMiddleware(tformat, lformat string, skip []string) echo.MiddlewareFunc {
	return middleware.LoggerWithConfig(middleware.LoggerConfig{
		Skipper:          SkipperRouteName(skip),
		Format:           lformat,
		CustomTimeFormat: tformat,
	})
}

func GzipMiddleware(level int) echo.MiddlewareFunc {
	return middleware.GzipWithConfig(middleware.GzipConfig{
		Skipper: middleware.DefaultSkipper,
		Level:   level,
	})
}

func ContextMiddleware(k string, i interface{}) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set(k, i)
			return next(c)
		}
	}
}

func ServerHeaderMiddleware(name string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Response().Header().Set(echo.HeaderServer, name)
			return next(c)
		}
	}
}

func CSRFMiddleware(cfg *Config) echo.MiddlewareFunc {
	return middleware.CSRFWithConfig(middleware.CSRFConfig{
		Skipper:        middleware.DefaultSkipper,
		TokenLength:    cfg.CSRF.TokenLength,
		TokenLookup:    cfg.CSRF.TokenLookup,
		ContextKey:     cfg.CSRF.ContextKey,
		CookieName:     cfg.CSRF.CookieName,
		CookieDomain:   cfg.Session.Domain,
		CookiePath:     cfg.Session.Path,
		CookieMaxAge:   cfg.Session.MaxAge,
		CookieSecure:   cfg.Session.Secure,
		CookieHTTPOnly: cfg.Session.HTTPOnly,
		CookieSameSite: cfg.Session.SameSite,
		ErrorHandler:   nil,
	})
}

func SessionMiddleware(cfg *Config) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			sess, err := redstore.Get(cfg.Session.SessID, c)
			if err != nil {
				logrus.Warnln("[SessionMiddleware]", "[redstore.Get]", err.Error())
				return next(c)
			}
			c.Set(CtxSession, sess)
			return next(c)
		}
	}
}

func StaticMiddleware(docroot string, fs *embed.FS) echo.MiddlewareFunc {
	return middleware.StaticWithConfig(middleware.StaticConfig{
		Skipper:    middleware.DefaultSkipper,
		Root:       docroot,
		Filesystem: http.FS(fs),
	})
}

func LastModifiedMiddleware(t *time.Time) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		loc, _ := time.LoadLocation("GMT")
		return func(c echo.Context) error {
			if c.Path() == "/*" {
				c.Request().Header.Set("Last-Modified", t.In(loc).Format(time.RFC1123))
			}
			return next(c)
		}
	}
}

func SkipperRouteName(routeNames []string) func(c echo.Context) bool {
	return func(c echo.Context) bool {
		if len(routeNames) == 0 {
			return false
		}
		for _, r := range c.Echo().Routes() {
			if slices.Contains(routeNames, r.Name) {
				return true
			}
		}
		return false
	}
}
