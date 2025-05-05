package echocore

import (
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/mrccnt/echocore/redstore"
	"github.com/sirupsen/logrus"
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
		Skipper: func(_ echo.Context) bool {
			// return strings.Contains(c.SelfURL(), "metrics") // Change "metrics" for your own path
			return false
		},
		Level: level,
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

func SkipperRouteName(routeNames []string) func(c echo.Context) bool {
	return func(c echo.Context) bool {
		if len(routeNames) == 0 {
			return false
		}
		for _, r := range c.Echo().Routes() {
			if r.Method == c.Request().Method && r.Path == c.Path() {
				for _, key := range routeNames {
					if key == r.Name {
						return true
					}
				}
			}
		}
		return false
	}
}
