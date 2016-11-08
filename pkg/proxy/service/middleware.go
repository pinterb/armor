package service

// This file contains the service middlewares.

import (
	"time"

	"golang.org/x/net/context"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/metrics"
)

// Middleware describes a service (as opposed to endpoint) middleware. It's
// a chainable behavior modifier for Service
type Middleware func(Service) Service

// LoggingMiddleware takes a logger as a dependency and returns a service
// middleware.
func LoggingMiddleware(logger log.Logger) Middleware {
	return func(next Service) Service {
		return loggingMiddleware{
			logger: logger,
			next:   next,
		}
	}
}

type loggingMiddleware struct {
	logger log.Logger
	next   Service
}

func (mw loggingMiddleware) InitStatus(ctx context.Context) (v bool, err error) {
	defer func() {
		mw.logger.Log(
			"method", "InitStatus",
			"result", v,
			"error", err,
		)
	}()
	return mw.next.InitStatus(ctx)
}

// InstrumentingMiddleware returns a service middleware that instruments
// requests made over the lifetime of the service.
func InstrumentingMiddleware(requestCount metrics.Counter, requestLatency metrics.Histogram) Middleware {
	return func(next Service) Service {
		return instrumentingMiddleware{
			requestCount:   requestCount,
			requestLatency: requestLatency,
			next:           next,
		}
	}
}

type instrumentingMiddleware struct {
	requestCount   metrics.Counter
	requestLatency metrics.Histogram
	next           Service
}

func (mw instrumentingMiddleware) InitStatus(ctx context.Context) (bool, error) {
	defer func(begin time.Time) {
		lvs := []string{"method", "initstatus", "error", "false"}
		mw.requestCount.With(lvs...).Add(1)
		mw.requestLatency.With(lvs...).Observe(time.Since(begin).Seconds())
	}(time.Now())

	v, err := mw.next.InitStatus(ctx)
	return v, err
}
