package redstore

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/gob"
	"errors"
	"math/big"
	"net/http"
	"time"

	"github.com/gorilla/sessions"
	"github.com/redis/go-redis/v9"
)

const (
	defaultMaxAge = 86400 * 30
	defaultPath   = "/"
	keyPrefix     = "session:"
)

// RedisStore stores gorilla sessions in Redis
type RedisStore struct {
	// client to connect to redis
	client redis.UniversalClient
	// default options to use when a new session is created
	options sessions.Options
	// key prefix with which the session will be stored
	keyPrefix string
	// key generator
	keyGen KeyGenFunc
	// session serializer
	serializer SessionSerializer
}

type KeyGenFunc func() (string, error)

// NewRedisStore returns a new RedisStore with default configuration
func NewRedisStore(ctx context.Context, client redis.UniversalClient) (*RedisStore, error) {
	rs := &RedisStore{
		options: sessions.Options{
			Path:   defaultPath,
			MaxAge: defaultMaxAge,
		},
		client:    client,
		keyPrefix: keyPrefix,
		keyGen: func() (string, error) {
			const (
				n       = 64
				letters = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz-"
			)
			ret := make([]byte, n)
			for i := 0; i < n; i++ {
				num, err := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
				if err != nil {
					return "", err
				}
				ret[i] = letters[num.Int64()]
			}
			return string(ret), nil
		},
		serializer: GobSerializer{},
	}

	return rs, rs.client.Ping(ctx).Err()
}

// Get returns a session for the given name after adding it to the registry.
func (s *RedisStore) Get(r *http.Request, name string) (*sessions.Session, error) {
	return sessions.GetRegistry(r).Get(s, name)
}

// New returns a session for the given name without adding it to the registry.
// nolint: errorlint
func (s *RedisStore) New(r *http.Request, name string) (*sessions.Session, error) {
	session := sessions.NewSession(s, name)
	opts := s.options
	session.Options = &opts
	session.IsNew = true

	c, err := r.Cookie(name)
	if err != nil {
		return session, nil
	}
	session.ID = c.Value

	err = s.load(r.Context(), session)
	if err == nil {
		session.IsNew = false
	} else if err == redis.Nil {
		err = nil // no data stored
	}
	return session, err
}

// Save adds a single session to the response.
func (s *RedisStore) Save(r *http.Request, w http.ResponseWriter, session *sessions.Session) error {
	if session.Options.MaxAge < 0 {
		if err := s.delete(r.Context(), session); err != nil {
			return err
		}
		http.SetCookie(w, sessions.NewCookie(session.Name(), "", session.Options))
		return nil
	}

	if session.ID == "" {
		id, err := s.keyGen()
		if err != nil {
			return errors.New("redisstore: failed to generate session id")
		}
		session.ID = id
	}
	if err := s.save(r.Context(), session); err != nil {
		return err
	}

	http.SetCookie(w, sessions.NewCookie(session.Name(), session.ID, session.Options))
	return nil
}

// Options set options to use when a new session is created
func (s *RedisStore) Options(opts sessions.Options) {
	s.options = opts
}

// KeyPrefix sets the key prefix to store session in Redis
func (s *RedisStore) KeyPrefix(keyPrefix string) {
	s.keyPrefix = keyPrefix
}

// KeyGen sets the key generator function
func (s *RedisStore) KeyGen(f KeyGenFunc) {
	s.keyGen = f
}

// Serializer sets the session serializer to store session
func (s *RedisStore) Serializer(ss SessionSerializer) {
	s.serializer = ss
}

// Close closes the Redis store
func (s *RedisStore) Close() error {
	return s.client.Close()
}

// save writes session in Redis
func (s *RedisStore) save(ctx context.Context, session *sessions.Session) error {
	b, err := s.serializer.Serialize(session)
	if err != nil {
		return err
	}

	return s.client.Set(ctx, s.keyPrefix+session.ID, b, time.Duration(session.Options.MaxAge)*time.Second).Err()
}

// load reads session from Redis
func (s *RedisStore) load(ctx context.Context, session *sessions.Session) error {
	cmd := s.client.Get(ctx, s.keyPrefix+session.ID)
	if cmd.Err() != nil {
		return cmd.Err()
	}

	b, err := cmd.Bytes()
	if err != nil {
		return err
	}

	return s.serializer.Deserialize(b, session)
}

// delete deletes session in Redis
func (s *RedisStore) delete(ctx context.Context, session *sessions.Session) error {
	return s.client.Del(ctx, s.keyPrefix+session.ID).Err()
}

// SessionSerializer provides an interface for serialize/deserialize a session
type SessionSerializer interface {
	Serialize(s *sessions.Session) ([]byte, error)
	Deserialize(b []byte, s *sessions.Session) error
}

// GobSerializer ...
type GobSerializer struct{}

func (gs GobSerializer) Serialize(s *sessions.Session) ([]byte, error) {
	buf := new(bytes.Buffer)
	enc := gob.NewEncoder(buf)
	err := enc.Encode(s.Values)
	if err == nil {
		return buf.Bytes(), nil
	}
	return nil, err
}

func (gs GobSerializer) Deserialize(d []byte, s *sessions.Session) error {
	dec := gob.NewDecoder(bytes.NewBuffer(d))
	return dec.Decode(&s.Values)
}
