// Package tracectx формирует и распространяет идентификаторы трассировки W3C
// (traceparent) без внешнего SDK: экспорт спанов в проекте выключен (ADR-030),
// а от трассировки требуются только trace_id и span_id в логах и метаданных gRPC.
package tracectx

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
)

// Header — имя заголовка/метаданных W3C Trace Context.
const Header = "traceparent"

// IDs — пара идентификаторов текущей операции.
type IDs struct {
	TraceID string // 32 hex
	SpanID  string // 16 hex
}

// New создаёт новые идентификаторы корня трассировки.
func New() IDs {
	var t [16]byte
	var s [8]byte
	_, _ = rand.Read(t[:])
	_, _ = rand.Read(s[:])
	return IDs{TraceID: hex.EncodeToString(t[:]), SpanID: hex.EncodeToString(s[:])}
}

// NewChild создаёт новый span в рамках существующей трассировки.
func NewChild(traceID string) IDs {
	var s [8]byte
	_, _ = rand.Read(s[:])
	if !validTrace(traceID) {
		return New()
	}
	return IDs{TraceID: traceID, SpanID: hex.EncodeToString(s[:])}
}

// Parse разбирает значение traceparent. При некорректном значении возвращает false.
func Parse(v string) (IDs, bool) {
	parts := strings.Split(v, "-")
	if len(parts) != 4 || parts[0] != "00" || !validTrace(parts[1]) || !validSpan(parts[2]) {
		return IDs{}, false
	}
	return IDs{TraceID: parts[1], SpanID: parts[2]}, true
}

// Format возвращает значение заголовка traceparent для исходящего вызова.
func (i IDs) Format() string {
	return fmt.Sprintf("00-%s-%s-01", i.TraceID, i.SpanID)
}

func validTrace(s string) bool { return isHex(s, 32) && strings.Trim(s, "0") != "" }

func validSpan(s string) bool { return isHex(s, 16) && strings.Trim(s, "0") != "" }

func isHex(s string, n int) bool {
	if len(s) != n {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}
