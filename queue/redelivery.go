package queue

import (
	"strconv"

	"github.com/pkg/errors"
	amqp "github.com/rabbitmq/amqp091-go"
)

const redeliveredCountHeader = "x-redelivered-count"

func redeliveryCount(delivery amqp.Delivery) (int, error) {
	value, found := delivery.Headers[redeliveredCountHeader]
	if !found {
		return 0, nil
	}

	switch v := value.(type) {
	case string:
		count, err := strconv.Atoi(v)
		if err != nil {
			return 0, errors.Wrapf(err, "failed to convert %s to int", redeliveredCountHeader)
		}

		return count, nil
	case int32:
		return int(v), nil
	default:
		return 0, errors.Errorf("unexpected type %T for %s header", value, redeliveredCountHeader)
	}
}

func redeliveryHeaders(count int) amqp.Table {
	if count == 0 {
		return nil
	}

	return amqp.Table{redeliveredCountHeader: count}
}
