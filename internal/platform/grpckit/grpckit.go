// Package grpckit — перехватчики gRPC: корреляция (W3C traceparent), логи,
// метрики, восстановление после паники и ограничение времени обработки.
package grpckit

import (
	"context"
	"log/slog"
	"runtime/debug"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"vovremya/internal/platform/logging"
	"vovremya/internal/platform/metrics"
	"vovremya/internal/platform/tracectx"
)

// UnaryServerInterceptor возвращает серверный перехватчик.
// maxHandlerTime ограничивает обработку, если клиент не задал более строгий deadline.
func UnaryServerInterceptor(log *slog.Logger, m *metrics.Registry, maxHandlerTime time.Duration) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		started := time.Now()
		ids := tracectx.New()
		requestID := ids.SpanID
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			if vals := md.Get(tracectx.Header); len(vals) > 0 {
				if parsed, ok := tracectx.Parse(vals[0]); ok {
					ids = tracectx.NewChild(parsed.TraceID)
				}
			}
			if vals := md.Get("x-request-id"); len(vals) > 0 && vals[0] != "" {
				requestID = vals[0]
			}
		}
		ctx = logging.WithCorrelation(ctx, requestID, ids.TraceID, ids.SpanID)
		ctx = logging.WithOperation(ctx, info.FullMethod)
		if _, hasDeadline := ctx.Deadline(); !hasDeadline && maxHandlerTime > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, maxHandlerTime)
			defer cancel()
		}

		defer func() {
			if r := recover(); r != nil {
				logging.From(ctx, log).Error("panic in grpc handler",
					slog.Any("panic", r), slog.String("stack", string(debug.Stack())))
				err = status.Error(codes.Internal, "internal error")
				resp = nil
				m.AppErrors.WithLabelValues("panic").Inc()
			}
			code := status.Code(err)
			m.GRPCServerHandled.WithLabelValues(info.FullMethod, code.String()).Inc()
			m.GRPCDuration.WithLabelValues(info.FullMethod, "server").Observe(time.Since(started).Seconds())
			result := "ok"
			switch {
			case code == codes.OK:
			case code == codes.Internal || code == codes.Unavailable || code == codes.DataLoss:
				result = "server_error"
			default:
				result = "client_error"
			}
			lg := logging.From(ctx, log).With(
				slog.Float64("duration_ms", float64(time.Since(started).Microseconds())/1000),
				slog.String("result", result),
				slog.String("error_code", code.String()))
			if result == "server_error" {
				lg.Error("grpc call")
			} else {
				lg.Info("grpc call")
			}
		}()
		return handler(ctx, req)
	}
}

// UnaryClientInterceptor возвращает клиентский перехватчик с корреляцией и метриками.
func UnaryClientInterceptor(m *metrics.Registry) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		started := time.Now()
		ids := tracectx.NewChild(logging.TraceIDFromContext(ctx))
		ctx = metadata.AppendToOutgoingContext(ctx, tracectx.Header, ids.Format())
		if rid := logging.RequestID(ctx); rid != "" {
			ctx = metadata.AppendToOutgoingContext(ctx, "x-request-id", rid)
		}
		err := invoker(ctx, method, req, reply, cc, opts...)
		m.GRPCClientRequests.WithLabelValues(method, status.Code(err).String()).Inc()
		m.GRPCDuration.WithLabelValues(method, "client").Observe(time.Since(started).Seconds())
		return err
	}
}
