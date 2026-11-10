package queue

import (
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedeliveryCount(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		headers amqp.Table
		want    int
	}{
		"no headers":   {headers: nil, want: 0},
		"no count":     {headers: amqp.Table{"other": 1}, want: 0},
		"int32 count":  {headers: amqp.Table{redeliveredCountHeader: int32(2)}, want: 2},
		"string count": {headers: amqp.Table{redeliveredCountHeader: "3"}, want: 3},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := redeliveryCount(amqp.Delivery{Headers: tt.headers})

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRedeliveryCount_Invalid(t *testing.T) {
	t.Parallel()

	for name, value := range map[string]any{"bad string": "x", "bad type": 1.5} {
		_, err := redeliveryCount(amqp.Delivery{Headers: amqp.Table{redeliveredCountHeader: value}})

		assert.Error(t, err, name)
	}
}

func TestRedeliveryHeaders_OmitsHeaderForZero(t *testing.T) {
	t.Parallel()

	assert.Nil(t, redeliveryHeaders(0))
	assert.Equal(t, amqp.Table{redeliveredCountHeader: 2}, redeliveryHeaders(2))
}
