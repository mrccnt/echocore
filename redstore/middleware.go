package redstore

import (
	"fmt"
	"github.com/gorilla/context"
	"github.com/gorilla/sessions"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

type Config struct {
	Skipper middleware.Skipper
	Store   sessions.Store
}

const key = "_session_store"

var DefaultConfig = Config{Skipper: middleware.DefaultSkipper}

func Get(name string, c echo.Context) (*sessions.Session, error) {
	store, err := getstore(c)
	if err != nil {
		return nil, err
	}
	return store.Get(c.Request(), name)
}

func New(name string, c echo.Context) (*sessions.Session, error) {
	store, err := getstore(c)
	if err != nil {
		return nil, err
	}
	return store.New(c.Request(), name)
}

func Middleware(store sessions.Store) echo.MiddlewareFunc {
	c := DefaultConfig
	c.Store = store
	return MiddlewareWithConfig(c)
}

func MiddlewareWithConfig(config Config) echo.MiddlewareFunc {
	if config.Skipper == nil {
		config.Skipper = DefaultConfig.Skipper
	}
	if config.Store == nil {
		panic("echo: session middleware requires store")
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if config.Skipper(c) {
				return next(c)
			}
			defer context.Clear(c.Request())
			c.Set(key, config.Store)
			return next(c)
		}
	}
}

func getstore(c echo.Context) (sessions.Store, error) {
	s := c.Get(key)
	if s == nil {
		return nil, fmt.Errorf("%q session store not found", key)
	}
	return s.(sessions.Store), nil
}
