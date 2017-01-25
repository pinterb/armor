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

func (mw loggingMiddleware) InitStatus(ctx context.Context) (initialized bool, err error) {
	defer func() {
		mw.logger.Log(
			"method", "InitStatus",
			"result", initialized,
			"error", err,
		)
	}()
	return mw.next.InitStatus(ctx)
}

func (mw loggingMiddleware) Init(ctx context.Context, opts InitOptions) (resp InitKeys, err error) {
	defer func() {
		mw.logger.Log(
			"method", "Init",
			"result", InitKeys{},
			"error", err,
		)
	}()
	return mw.next.Init(ctx, opts)
}

func (mw loggingMiddleware) SealStatus(ctx context.Context) (resp SealState, err error) {
	defer func() {
		mw.logger.Log(
			"method", "SealStatus",
			"result", SealState{},
			"error", err,
		)
	}()
	return mw.next.SealStatus(ctx)
}

func (mw loggingMiddleware) Unseal(ctx context.Context, opts UnsealOptions) (resp SealState, err error) {
	defer func() {
		mw.logger.Log(
			"method", "Unseal",
			"result", SealState{},
			"error", err,
		)
	}()
	return mw.next.Unseal(ctx, opts)
}

func (mw loggingMiddleware) Configure(ctx context.Context, opts ConfigOptions) (resp ConfigState, err error) {
	defer func() {
		mw.logger.Log(
			"method", "Configure",
			"result", ConfigState{},
			"error", err,
		)
	}()
	return mw.next.Configure(ctx, opts)
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

	initialized, err := mw.next.InitStatus(ctx)
	return initialized, err
}

func (mw instrumentingMiddleware) Init(ctx context.Context, opts InitOptions) (resp InitKeys, err error) {
	defer func(begin time.Time) {
		lvs := []string{"method", "init", "error", "false"}
		mw.requestCount.With(lvs...).Add(1)
		mw.requestLatency.With(lvs...).Observe(time.Since(begin).Seconds())
	}(time.Now())

	resp, err = mw.next.Init(ctx, opts)
	return resp, err
}

func (mw instrumentingMiddleware) SealStatus(ctx context.Context) (resp SealState, err error) {
	defer func(begin time.Time) {
		lvs := []string{"method", "sealstatus", "error", "false"}
		mw.requestCount.With(lvs...).Add(1)
		mw.requestLatency.With(lvs...).Observe(time.Since(begin).Seconds())
	}(time.Now())

	resp, err = mw.next.SealStatus(ctx)
	return resp, err
}

func (mw instrumentingMiddleware) Unseal(ctx context.Context, opts UnsealOptions) (resp SealState, err error) {
	defer func(begin time.Time) {
		lvs := []string{"method", "unseal", "error", "false"}
		mw.requestCount.With(lvs...).Add(1)
		mw.requestLatency.With(lvs...).Observe(time.Since(begin).Seconds())
	}(time.Now())

	resp, err = mw.next.Unseal(ctx, opts)
	return resp, err
}

func (mw instrumentingMiddleware) Configure(ctx context.Context, opts ConfigOptions) (resp ConfigState, err error) {
	defer func(begin time.Time) {
		lvs := []string{"method", "configure", "error", "false"}
		mw.requestCount.With(lvs...).Add(1)
		mw.requestLatency.With(lvs...).Observe(time.Since(begin).Seconds())
	}(time.Now())

	resp, err = mw.next.Configure(ctx, opts)
	return resp, err
}
