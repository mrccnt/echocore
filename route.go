package echocore

import (
	"github.com/labstack/echo/v4"
	"github.com/mrccnt/echocore/redstore"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"net/http"
)

const (
	CtxCore    = "core"
	CtxSession = "session"
)

type Handler interface {
	Init() error
	Exec() error
	Error(err error) error
}

type Route struct {
	Ctx echo.Context
}

type ServiceMessage struct {
	Message string `json:"message"`
}

func NewRoute(ctx echo.Context) Route {
	return Route{ctx}
}

func Handle(h Handler) error {
	if err := h.Init(); err != nil {
		return h.Error(err)
	}
	return h.Exec()
}

func (r *Route) BindVal(i interface{}) error {
	if err := r.Ctx.Bind(i); err != nil {
		return err
	}
	return r.Ctx.Validate(i)
}

func (r *Route) Bind(i interface{}) error {

	var err error

	if err = (&echo.DefaultBinder{}).BindBody(r.Ctx, i); err != nil {
		return err
	}

	if err = (&echo.DefaultBinder{}).BindHeaders(r.Ctx, i); err != nil {
		return err
	}

	if err = (&echo.DefaultBinder{}).BindQueryParams(r.Ctx, i); err != nil {
		return err
	}

	if err = (&echo.DefaultBinder{}).BindPathParams(r.Ctx, i); err != nil {
		return err
	}

	return nil
}

func (r *Route) Config() *Config {
	return r.Ctx.Get(CtxCore).(*Core).Config
}

func (r *Route) Gorm() *gorm.DB {
	return r.Ctx.Get(CtxCore).(*Core).Gorm
}

func (r *Route) Redis() *redis.Client {
	return r.Ctx.Get(CtxCore).(*Core).Redis
}

func (r *Route) SessStore() *redstore.RedisStore {
	return r.Ctx.Get(CtxCore).(*Core).SessStore
}

func (r *Route) Error(err error) error {
	logrus.Errorln(err.Error())
	return r.Ctx.JSON(http.StatusInternalServerError, &ServiceMessage{Message: http.StatusText(http.StatusInternalServerError)})
}

func (r *Route) BadRequest(err error) error {
	logrus.Warnln(err.Error())
	return r.Ctx.JSON(http.StatusBadRequest, &ServiceMessage{Message: err.Error()})
}
