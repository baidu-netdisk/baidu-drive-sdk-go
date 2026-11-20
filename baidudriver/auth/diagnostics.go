package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"sync/atomic"
	"time"
)

type diagnosticLogger struct {
	attemptID string
	logger    *log.Logger
}

var diagnosticFallbackSequence atomic.Uint64

func newDiagnosticLogger(enabled bool, writer io.Writer) *diagnosticLogger {
	if !enabled {
		return nil
	}
	if writer == nil {
		writer = os.Stderr
	}
	attemptID, err := randomURLSafe(12)
	if err != nil {
		attemptID = fmt.Sprintf("local-%x-%x", time.Now().UnixNano(), diagnosticFallbackSequence.Add(1))
	}
	return &diagnosticLogger{
		attemptID: attemptID,
		logger:    log.New(writer, "[baidupan auth] ", log.LstdFlags|log.Lmicroseconds|log.LUTC),
	}
}

func (d *diagnosticLogger) log(event, format string, args ...any) {
	if d == nil {
		return
	}
	message := ""
	if format != "" {
		message = " " + fmt.Sprintf(format, args...)
	}
	d.logger.Printf("attempt_id=%s event=%s%s", d.attemptID, event, message)
}

func diagnosticHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:6])
}

func diagnosticDurationMS(start time.Time) int64 {
	return time.Since(start).Milliseconds()
}

func diagnosticSafeText(value string, maxLength int, secrets ...string) string {
	return safeText(redactValues(value, secrets...), maxLength)
}
