package echocore

import (
	"crypto/tls"
	"crypto/x509"
	"github.com/labstack/gommon/log"
	"github.com/sirupsen/logrus"
	"net/http"
	"os"
)

const (
	logDebug = "debug"
	logInfo  = "info"
	logWarn  = "warn"
	logError = "error"
)

type Config struct {
	App struct {
		Bind         string `json:"bind"          env:"APP_BIND"          envDefault:":8082"        validate:"required"`
		EchoTimeout  int    `json:"echo_timeout"  env:"APP_ECHO_TIMEOUT"  envDefault:"10"           validate:"required,gte=0"`
		GzipCompr    int    `json:"gzip_compr"    env:"APP_GZIP_COMPR"    envDefault:"-1"           validate:"gzip_compr"`
		ServerHeader string `json:"server_header" env:"APP_SERVER_HEADER" envDefault:"echocore/1.0"`
	} `json:"app"`
	Log struct {
		Level      string   `json:"level"       env:"LOG_LEVEL"       envDefault:"info" validate:"log_level"`
		TimeFormat string   `json:"time_format" env:"LOG_TIME_FORMAT" envDefault:"2006-01-02T15:04:05Z07:00" validate:"required"`
		LineFormat string   `json:"line_format" env:"LOG_LINE_FORMAT" envDefault:"ANSWER [${time_custom}] [${id}] [${status}] ${method} ${uri} ${error}\n" validate:"required"`
		SkipRoutes []string `json:"skip_routes" env:"LOG_SKIP_ROUTES"`
	} `json:"log"`
	DB struct {
		Addr      string `json:"addr"       env:"DB_ADDR"            envDefault:"localhost:3306" validate:"hostname_port"`
		User      string `json:"user"       env:"DB_USER"            envDefault:""`
		Pass      string `json:"pass"       env:"DB_PASS"            envDefault:""`
		Name      string `json:"name"       env:"DB_NAME"            envDefault:""`
		Timezone  string `json:"timezone"   env:"DB_TIMEZONE"        envDefault:"Europe/Berlin"  validate:"timezone"`
		Collation string `json:"collation"  env:"DB_COLLATION"       envDefault:"utf8mb4_unicode_ci"`
		Charset   string `json:"charset"    env:"DB_CHARSET"         envDefault:"utf8mb4"`
		ParseTime bool   `json:"parse_time" env:"DB_PARSE_TIME"      envDefault:"true"`
		Multi     bool   `json:"multi"      env:"DB_MULTI_STATEMENT" envDefault:"true"`
		MaxIdle   int    `json:"max_idle"   env:"DB_MAX_IDLE"        envDefault:"10"`
		MaxOpen   int    `json:"max_open"   env:"DB_MAX_OPEN"        envDefault:"50"`
		MaxLife   int    `json:"max_life"   env:"DB_MAX_LIFE"        envDefault:"60"`
		TLS       struct {
			Crt        string             `json:"crt"         env:"DB_TLS_CRT"         envDefault:""    validate:"omitempty,file"`
			Key        string             `json:"key"         env:"DB_TLS_KEY"         envDefault:""    validate:"omitempty,file"`
			ClientCAs  []string           `json:"client_cas"  env:"DB_TLS_CLIENT_CAS"`
			RootCAs    []string           `json:"root_cas"    env:"DB_TLS_ROOT_CAS"`
			SkipVerify bool               `json:"skip_verify" env:"DB_TLS_SKIP_VERIFY"`
			ClientAuth tls.ClientAuthType `json:"client_auth" env:"DB_TLS_CLIENT_AUTH" envDefault:"0"   validate:"client_auth"`
			MinVersion uint16             `json:"min_version" env:"DB_TLS_MIN_VERSION" envDefault:"771" validate:"tls_ver"`
		} `json:"tls"`
	} `json:"db"`
	Redis struct {
		Addr string `json:"addr" env:"REDIS_ADDR" envDefault:"localhost:6379" validate:"hostname_port"`
		User string `json:"user" env:"REDIS_USER" envDefault:""`
		Pass string `json:"pass" env:"REDIS_PASS" envDefault:""`
		TLS  struct {
			Crt        string             `json:"crt"         env:"REDIS_TLS_CRT"         envDefault:""    validate:"omitempty,file"`
			Key        string             `json:"key"         env:"REDIS_TLS_KEY"         envDefault:""    validate:"omitempty,file"`
			ClientCAs  []string           `json:"client_cas"  env:"REDIS_TLS_CLIENT_CAS"`
			RootCAs    []string           `json:"root_cas"    env:"REDIS_TLS_ROOT_CAS"`
			SkipVerify bool               `json:"skip_verify" env:"REDIS_TLS_SKIP_VERIFY"`
			ClientAuth tls.ClientAuthType `json:"client_auth" env:"REDIS_TLS_CLIENT_AUTH" envDefault:"0"   validate:"client_auth"`
			MinVersion uint16             `json:"min_version" env:"REDIS_TLS_MIN_VERSION" envDefault:"771" validate:"tls_ver"`
		} `json:"tls"`
	} `json:"redis"`
	Session struct {
		Path     string        `json:"path"      env:"SESS_PATH"         envDefault:"/"         validate:"required,gte=1"`
		Domain   string        `json:"domain"    env:"SESS_DOMAIN"       envDefault:"localhost" validate:"required"`
		MaxAge   int           `json:"max_age"   env:"SESS_MAX_AGE"      envDefault:"0"`
		Secure   bool          `json:"secure"    env:"SESS_SECURE"       envDefault:"false"`
		HTTPOnly bool          `json:"http_only" env:"SESS_HTTP_ONLY"    envDefault:"true"`
		SameSite http.SameSite `json:"same_site" env:"SESS_SAME_SITE"    envDefault:"1"         validate:"required,gte=1,lte=4"`
		SessID   string        `json:"sess_id"   env:"SESS_SESS_ID"      envDefault:"id"        validate:"required,gte=1,lte=64"`
		Seconds  int           `json:"seconds"   env:"SESS_SESS_SECONDS" envDefault:"600"       validate:"required,gte=1"`
	} `json:"session"`
	CSRF struct {
		TokenLength uint8  `json:"token_length" env:"CSRF_TOKEN_LENGTH" envDefault:"32"        validate:"gte=12"`
		TokenLookup string `json:"token_lookup" env:"CSRF_TOKEN_LOOKUP" envDefault:"form:csrf" validate:"required"`
		ContextKey  string `json:"context_key"  env:"CSRF_CONTEXT_KEY"  envDefault:"csrf"      validate:"required"`
		CookieName  string `json:"cookie_name"  env:"CSRF_COOKIE_NAME"  envDefault:"idc"       validate:"required"`
	} `json:"csrf"`
}

func (cfg *Config) GommonLevel() log.Lvl {
	switch cfg.Log.Level {
	case logDebug:
		return log.DEBUG
	case logInfo:
		return log.INFO
	case logWarn:
		return log.WARN
	case logError:
		return log.ERROR
	default:
		panic("not a valid gommon Level: " + cfg.Log.Level)
	}
}

func (cfg *Config) LogrusLevel() logrus.Level {
	lvl, err := logrus.ParseLevel(cfg.Log.Level)
	if err != nil {
		panic(err)
	}
	return lvl
}

func (cfg *Config) TLSConfigDB() (*tls.Config, error) {
	return mTLS(&tlsConfig{
		Crt:                cfg.DB.TLS.Crt,
		Key:                cfg.DB.TLS.Key,
		ClientCAs:          cfg.DB.TLS.ClientCAs,
		RootCAs:            cfg.DB.TLS.RootCAs,
		InsecureSkipVerify: cfg.DB.TLS.SkipVerify,
		ClientAuth:         cfg.DB.TLS.ClientAuth,
		MinVersion:         cfg.DB.TLS.MinVersion,
	})
}

func (cfg *Config) TLSConfigRedis() (*tls.Config, error) {
	return mTLS(&tlsConfig{
		Crt:                cfg.Redis.TLS.Crt,
		Key:                cfg.Redis.TLS.Key,
		ClientCAs:          cfg.Redis.TLS.ClientCAs,
		RootCAs:            cfg.Redis.TLS.RootCAs,
		InsecureSkipVerify: cfg.Redis.TLS.SkipVerify,
		ClientAuth:         cfg.Redis.TLS.ClientAuth,
		MinVersion:         cfg.Redis.TLS.MinVersion,
	})
}

func (cfg *Config) IsTLSConfiguredDB() bool {
	return cfg.DB.TLS.Crt != "" && cfg.DB.TLS.Key != ""
}

func (cfg *Config) IsTLSConfiguredRedis() bool {
	return cfg.Redis.TLS.Crt != "" && cfg.Redis.TLS.Key != ""
}

func (cfg *Config) IsDebug() bool {
	return cfg.Log.Level == logDebug
}

type tlsConfig struct {
	Crt                string
	Key                string
	ClientCAs          []string
	RootCAs            []string
	InsecureSkipVerify bool
	ClientAuth         tls.ClientAuthType
	MinVersion         uint16
}

func mTLS(cfg *tlsConfig) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(cfg.Crt, cfg.Key)
	if err != nil {
		return nil, err
	}
	return &tls.Config{
		// nolint: gosec
		InsecureSkipVerify: cfg.InsecureSkipVerify,
		ClientAuth:         cfg.ClientAuth,
		MinVersion:         cfg.MinVersion,
		Certificates:       []tls.Certificate{cert},
		RootCAs:            mPool(cfg.RootCAs),
		ClientCAs:          mPool(cfg.ClientCAs),
	}, nil
}

func mPool(files []string) *x509.CertPool {
	p := x509.NewCertPool()
	for _, f := range files {
		bs, err := os.ReadFile(f)
		if err != nil {
			logrus.Warnf("[makePool] [os.ReadFile] %s\n", err.Error())
			continue
		}
		if ok := p.AppendCertsFromPEM(bs); !ok {
			logrus.Warnf("[makePool] [AppendCertsFromPEM] %s\n", "not ok")
			continue
		}
	}
	return p
}
