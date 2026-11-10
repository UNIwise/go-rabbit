package client

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

const defaultPort = 5672

// ParseDSN parses an amqp DSN like amqp://user:password@host:5672/vhost into a Config.
// The port defaults to 5672. Errors never contain the DSN or its credentials.
func ParseDSN(dsn string) (*Config, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return nil, errors.New("dsn is not a valid url")
	}

	if u.Scheme != "amqp" {
		return nil, errors.Errorf("invalid scheme %q, expected 'amqp'", u.Scheme)
	}

	port, err := parsePort(u.Port())
	if err != nil {
		return nil, err
	}

	if u.User == nil || u.User.Username() == "" {
		return nil, errors.New("dsn is missing a user")
	}

	password, hasPassword := u.User.Password()
	if !hasPassword {
		return nil, errors.New("dsn is missing a password")
	}

	return &Config{
		Host:     u.Hostname(),
		Port:     port,
		User:     u.User.Username(),
		Password: password,
		VHost:    strings.TrimPrefix(u.Path, "/"),
	}, nil
}

func parsePort(raw string) (uint32, error) {
	if raw == "" {
		return defaultPort, nil
	}

	port, err := strconv.ParseUint(raw, 10, 16)
	if err != nil {
		return 0, errors.Errorf("invalid port %q", raw)
	}

	return uint32(port), nil
}
