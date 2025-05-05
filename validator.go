package echocore

import (
	"compress/gzip"
	"crypto/tls"
	impl "github.com/go-playground/validator/v10"
	"slices"
)

type CustomValidator struct {
	Validator *impl.Validate
}

func NewValidator() *CustomValidator {

	v := impl.New()

	_ = v.RegisterValidation("client_auth", func(fl impl.FieldLevel) bool {
		// 0 1 2 3 4
		switch tls.ClientAuthType(int(fl.Field().Int())) {
		case tls.NoClientCert,
			tls.RequestClientCert,
			tls.RequireAnyClientCert,
			tls.VerifyClientCertIfGiven,
			tls.RequireAndVerifyClientCert:
			return true
		default:
			return false
		}
	})

	_ = v.RegisterValidation("tls_ver", func(fl impl.FieldLevel) bool {
		// 769 770 771 772
		// nolint: gosec
		switch uint16(fl.Field().Uint()) {
		case tls.VersionTLS10,
			tls.VersionTLS11,
			tls.VersionTLS12,
			tls.VersionTLS13:
			return true
		default:
			return false
		}
	})

	_ = v.RegisterValidation("gzip_compr", func(fl impl.FieldLevel) bool {
		// 0 1 9 -1 -2
		switch int(fl.Field().Int()) {
		case gzip.NoCompression,
			gzip.BestSpeed,
			gzip.BestCompression,
			gzip.DefaultCompression,
			gzip.HuffmanOnly:
			return true
		default:
			return false
		}
	})

	_ = v.RegisterValidation("log_level", func(fl impl.FieldLevel) bool {
		return slices.Contains([]string{logDebug, logInfo, logWarn, logError}, fl.Field().String())
	})

	return &CustomValidator{Validator: v}
}

func (v *CustomValidator) Validate(i interface{}) error {
	return v.Validator.Struct(i)
}
