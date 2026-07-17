package gatewaysvc

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ddag/ddag/internal/logging"
	"github.com/ddag/ddag/internal/metrics"
	"github.com/ddag/ddag/internal/models"
)

// reqLogStore is the slice of *store.Store the request logger needs. An
// interface keeps the logger unit-testable without a live metadata DB.
type reqLogStore interface {
	InsertRequestLogBatch(ctx context.Context, rs []*models.APIRequestLog) error
}

// requestLogger persists data-plane access logs off the request path: handlers
// drop a record into a bounded channel (never blocking the response), and a
// single background flusher batches them into one multi-row INSERT per
// batchSize records or per flushEvery tick. This replaces a goroutine + INSERT
// per request, which otherwise spawns unbounded goroutines and saturates the
// gateway's metadata pool under load.
type requestLogger struct {
	store      reqLogStore
	log        *logging.Logger
	metrics    *metrics.Metrics
	ch         chan *models.APIRequestLog
	batchSize  int
	flushEvery time.Duration
	dropped    atomic.Int64
	wg         sync.WaitGroup
}

func newRequestLogger(st reqLogStore, log *logging.Logger, m *metrics.Metrics, bufferSize, batchSize int, flushEvery time.Duration) *requestLogger {
	if bufferSize <= 0 {
		bufferSize = 4096
	}
	if batchSize <= 0 {
		batchSize = 200
	}
	// Stay well under PostgreSQL's 65535 bound parameters (13 cols/row).
	if batchSize > 4000 {
		batchSize = 4000
	}
	if flushEvery <= 0 {
		flushEvery = time.Second
	}
	return &requestLogger{
		store: st, log: log, metrics: m,
		ch:         make(chan *models.APIRequestLog, bufferSize),
		batchSize:  batchSize,
		flushEvery: flushEvery,
	}
}

// start launches the flusher. It stops (after a final drain) when ctx is done.
func (l *requestLogger) start(ctx context.Context) {
	l.wg.Add(1)
	go l.run(ctx)
}

// stop waits for the flusher to drain and exit. Cancel start's ctx first.
func (l *requestLogger) stop() { l.wg.Wait() }

// enqueue hands a record to the flusher without ever blocking the request path.
// If the buffer is full the record is dropped and counted, so a slow metadata
// DB sheds logs instead of stalling responses.
func (l *requestLogger) enqueue(rec *models.APIRequestLog) {
	select {
	case l.ch <- rec:
	default:
		l.dropped.Add(1)
		if l.metrics != nil {
			l.metrics.RequestLogsDropped.Inc()
		}
	}
}

func (l *requestLogger) run(ctx context.Context) {
	defer l.wg.Done()
	t := time.NewTicker(l.flushEvery)
	defer t.Stop()
	batch := make([]*models.APIRequestLog, 0, l.batchSize)

	flush := func() {
		if len(batch) == 0 {
			return
		}
		// Use a fresh Background context: flushing must still work while the
		// service is shutting down (the run ctx is already cancelled by then).
		fctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := l.store.InsertRequestLogBatch(fctx, batch); err != nil {
			l.log.Warn("request_log_batch_failed", "count", len(batch), "error", err.Error())
		}
		cancel()
		batch = batch[:0]
	}
	add := func(rec *models.APIRequestLog) {
		batch = append(batch, rec)
		if len(batch) >= l.batchSize {
			flush()
		}
	}

	for {
		select {
		case <-ctx.Done():
			// Drain whatever is buffered, then flush the remainder and exit.
			for {
				select {
				case rec := <-l.ch:
					add(rec)
				default:
					flush()
					return
				}
			}
		case rec := <-l.ch:
			add(rec)
		case <-t.C:
			flush()
		}
	}
}
