package logging

import (
	"fmt"
	"time"

	"github.com/DataDog/datadog-go/statsd"
)

type Metrics interface {
	Timing(name string, duration time.Duration, tags []string)
}

type NullMetrics struct {
}

func (s *NullMetrics) Timing(name string, duration time.Duration, tags []string) {}

type StatsDMetrics struct {
	c *statsd.Client
}

func NewStatsDMetrics(serviceName, environment, uri string) Metrics {
	c, _ := statsd.New(uri)
	c.Tags = []string{
		fmt.Sprintf("service:%s", serviceName),
		fmt.Sprintf("env:%s", environment),
	}

	return &StatsDMetrics{
		c: c,
	}
}

func (s *StatsDMetrics) Timing(name string, duration time.Duration, tags []string) {
	s.c.Timing(name, duration, tags, 1)
}
