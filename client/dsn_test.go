package client_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uniwise/go-rabbit/v2/client"
)

func TestParseDSN(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		dsn  string
		want client.Config
	}{
		"all parts": {
			dsn:  "amqp://user:pass@rabbit.local:5673/vhost",
			want: client.Config{Host: "rabbit.local", Port: 5673, User: "user", Password: "pass", VHost: "vhost"},
		},
		"port defaults to 5672": {
			dsn:  "amqp://user:pass@rabbit.local/vhost",
			want: client.Config{Host: "rabbit.local", Port: 5672, User: "user", Password: "pass", VHost: "vhost"},
		},
		"trailing slash gives empty vhost": {
			dsn:  "amqp://user:pass@rabbit.local/",
			want: client.Config{Host: "rabbit.local", Port: 5672, User: "user", Password: "pass"},
		},
		"no path gives empty vhost": {
			dsn:  "amqp://user:pass@rabbit.local",
			want: client.Config{Host: "rabbit.local", Port: 5672, User: "user", Password: "pass"},
		},
		"percent-encoded password is decoded": {
			dsn:  "amqp://user:p%40ss%2Fw%3Ard@rabbit.local/vhost",
			want: client.Config{Host: "rabbit.local", Port: 5672, User: "user", Password: "p@ss/w:rd", VHost: "vhost"},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := client.ParseDSN(tt.dsn)

			require.NoError(t, err)
			assert.Equal(t, tt.want, *got)
		})
	}
}

func TestParseDSN_Invalid(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"wrong scheme":     "amqps://user:secret@rabbit.local/vhost",
		"no scheme":        "user:secret@rabbit.local/vhost",
		"missing userinfo": "amqp://rabbit.local/vhost",
		"missing user":     "amqp://:secret@rabbit.local/vhost",
		"missing password": "amqp://user@rabbit.local/vhost",
		"port not numeric": "amqp://user:secret@rabbit.local:abc/vhost",
		"port too large":   "amqp://user:secret@rabbit.local:70000/vhost",
		"unparsable url":   "amqp://user:secret@rabbit.local:5672%zz/vhost",
	}

	for name, dsn := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := client.ParseDSN(dsn)

			require.Error(t, err)
			assert.Nil(t, got)
			assert.NotContains(t, err.Error(), "secret")
		})
	}
}
