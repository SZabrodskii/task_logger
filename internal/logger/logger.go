package logger

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type key int

const loggerKey key = iota

func NewContext(ctx context.Context, l Logger) context.Context {
	return context.WithValue(ctx, loggerKey, l)
}

func FromContext(ctx context.Context) Logger {
	if l, ok := ctx.Value(loggerKey).(Logger); ok && l != nil {
		return l
	}

	return NewNoOpLogger()
}

type LogLevel int8

const (
	DebugLevel LogLevel = iota - 1
	InfoLevel
	WarnLevel
	ErrorLevel
	DPanicLevel
	PanicLevel
	FatalLevel
)

const (
	defaultBufferSize    = 4096
	defaultFlushInterval = 100 * time.Millisecond
)

type Config struct {
	Level        LogLevel
	IsProduction bool
}

type Logger interface {
	Debug(msg string, fields ...interface{})
	Info(msg string, fields ...interface{})
	Warn(msg string, fields ...interface{})
	Error(msg string, fields ...interface{})
	DPanic(msg string, fields ...interface{})
	Panic(msg string, fields ...interface{})
	Fatal(msg string, fields ...interface{})
	With(fields ...interface{}) Logger
	Close()
}

type asyncLogger struct {
	cfg           Config
	logChan       chan []byte
	wg            sync.WaitGroup
	writer        *log.Logger
	bufferSize    int
	flushInterval time.Duration
	contextFields []interface{}
}

type Option func(*asyncLogger)

func WithBufferSize(size int) Option {
	return func(l *asyncLogger) {
		if size > 0 {
			l.bufferSize = size
		}
	}
}

func WithFlushInterval(interval time.Duration) Option {
	return func(l *asyncLogger) {
		if interval > 0 {
			l.flushInterval = interval
		}
	}
}

func NewAsyncLogger(ctx context.Context, cfg Config, opts ...Option) Logger {
	l := &asyncLogger{
		cfg:           cfg,
		writer:        log.New(os.Stdout, "", 0),
		bufferSize:    defaultBufferSize,
		flushInterval: defaultFlushInterval,
	}

	for _, opt := range opts {
		opt(l)
	}

	l.logChan = make(chan []byte, l.bufferSize+1)

	l.wg.Add(1)
	go l.run(ctx)

	return l
}

func (l *asyncLogger) run(ctx context.Context) {
	defer l.wg.Done()
	ticker := time.NewTicker(l.flushInterval)
	defer ticker.Stop()

	var batch bytes.Buffer
	flush := func() {
		if batch.Len() > 0 {
			l.writer.Writer().Write(batch.Bytes())
			batch.Reset()
		}
	}

	for {
		select {
		case <-ctx.Done():
			for {
				select {
				case msg := <-l.logChan:
					batch.Write(msg)
				default:
					flush()
					return
				}
			}

		case <-ticker.C:
			flush()

		case msg, ok := <-l.logChan:
			if !ok {
				flush()
				return
			}
			batch.Write(msg)
			if batch.Len() >= l.bufferSize {
				flush()
			}
		}
	}
}

func (l *asyncLogger) Debug(msg string, fields ...interface{}) {
	if l.cfg.Level <= DebugLevel {
		l.log("DEBUG", msg, fields...)
	}
}

func (l *asyncLogger) Info(msg string, fields ...interface{}) {
	if l.cfg.Level <= InfoLevel {
		l.log("INFO", msg, fields...)
	}
}

func (l *asyncLogger) Warn(msg string, fields ...interface{}) {
	if l.cfg.Level <= WarnLevel {
		l.log("WARN", msg, fields...)
	}
}

func (l *asyncLogger) Error(msg string, fields ...interface{}) {
	if l.cfg.Level <= ErrorLevel {
		l.log("ERROR", msg, fields...)
	}
}

func (l *asyncLogger) DPanic(msg string, fields ...interface{}) {
	if l.cfg.Level <= DPanicLevel {
		l.log("DPANIC", msg, fields...)
		if !l.cfg.IsProduction {
			panic(msg)
		}
	}
}

func (l *asyncLogger) Panic(msg string, fields ...interface{}) {
	if l.cfg.Level <= PanicLevel {
		l.log("PANIC", msg, fields...)
		panic(msg)
	}

}

func (l *asyncLogger) Fatal(msg string, fields ...interface{}) {
	if l.cfg.Level <= FatalLevel {
		l.log("FATAL", msg, fields...)
		os.Exit(1)
	}
}

func (l *asyncLogger) With(fields ...interface{}) Logger {
	newLogger := *l
	newLogger.contextFields = make([]interface{}, len(l.contextFields))
	copy(newLogger.contextFields, l.contextFields)
	newLogger.contextFields = append(newLogger.contextFields, fields...)
	return &newLogger
}

func (l *asyncLogger) log(level, msg string, fields ...interface{}) {
	var sb strings.Builder
	sb.WriteString(time.Now().Format(time.RFC3339Nano))
	sb.WriteString(" ")
	sb.WriteString(level)
	sb.WriteString(" ")
	sb.WriteString(msg)

	allFields := append(l.contextFields, fields...)

	if len(allFields) > 0 {
		for i := 0; i < len(allFields); i += 2 {
			sb.WriteString(" ")
			key, ok := allFields[i].(string)
			if !ok {
				continue
			}
			sb.WriteString(key)
			sb.WriteString("=")
			if i+1 < len(allFields) {
				appendValue(&sb, allFields[i+1])
			}
		}
	}
	sb.WriteString("\n")

	select {
	case l.logChan <- []byte(sb.String()):
	default:
		fmt.Fprintf(os.Stderr, "WARNING: Logger channel is full. Log message dropped: %s\n", sb.String())
	}

}

func (l *asyncLogger) Close() {
	l.wg.Wait()
}

func appendValue(sb *strings.Builder, value interface{}) {
	switch val := value.(type) {
	case string:
		sb.WriteString(val)
	case int:
		sb.WriteString(strconv.Itoa(val))
	case int64:
		sb.WriteString(strconv.FormatInt(val, 10))
	case uint:
		sb.WriteString(strconv.FormatUint(uint64(val), 10))
	case uint64:
		sb.WriteString(strconv.FormatUint(val, 10))
	case float64:
		sb.WriteString(strconv.FormatFloat(val, 'f', -1, 64))
	case bool:
		sb.WriteString(strconv.FormatBool(val))
	case error:
		sb.WriteString(val.Error())
	default:
		sb.WriteString(fmt.Sprintf("%v", val))
	}
}

func ParseLevel(level string) LogLevel {
	switch strings.ToLower(level) {
	case "debug":
		return DebugLevel
	case "info":
		return InfoLevel
	case "warn", "warning":
		return WarnLevel
	case "error":
		return ErrorLevel
	case "dpanic":
		return DPanicLevel
	case "panic":
		return PanicLevel
	case "fatal":
		return FatalLevel
	default:
		return InfoLevel
	}
}

type noOpLogger struct{}

func (n *noOpLogger) Debug(msg string, fields ...interface{})  {}
func (n *noOpLogger) Info(msg string, fields ...interface{})   {}
func (n *noOpLogger) Warn(msg string, fields ...interface{})   {}
func (n *noOpLogger) Error(msg string, fields ...interface{})  {}
func (n *noOpLogger) DPanic(msg string, fields ...interface{}) {}
func (n *noOpLogger) Panic(msg string, fields ...interface{})  {}
func (n *noOpLogger) Fatal(msg string, fields ...interface{})  {}

func (n *noOpLogger) With(fields ...interface{}) Logger {
	return n
}
func (n *noOpLogger) Close() {}

func NewNoOpLogger() Logger {
	return &noOpLogger{}

}

func RequestLogger(baseLogger Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			traceparent := generateTraceparent()
			reqLogger := baseLogger.With("traceparent", traceparent)

			reqLogger.Info("request started", "method", r.Method, "path", r.URL.Path)
			start := time.Now()

			ctx := NewContext(r.Context(), reqLogger)
			next.ServeHTTP(w, r.WithContext(ctx))

			reqLogger.Info("request completed", "duration", time.Since(start))
		})
	}
}

var requests = atomic.Int64{}

func generateTraceID() string {
	hi := rand.Uint64()
	lo := rand.Uint64()
	return fmt.Sprintf("%016x%016x", hi, lo)
}
func generateSpanID() string {
	randomNum := rand.Uint64()
	return fmt.Sprintf("%016x", randomNum)
}

func generateTraceFlags() string {
	defer func() {
		requests.Add(1)
	}()

	if requests.Load()%100 == 0 {
		return "01"
	}

	return "00"
}

func generateTraceparent() string {
	version := "00"
	traceID := generateTraceID()
	spanID := generateSpanID()
	traceFlags := generateTraceFlags()

	return fmt.Sprintf("%s-%s-%s-%s", version, traceID, spanID, traceFlags)
}
