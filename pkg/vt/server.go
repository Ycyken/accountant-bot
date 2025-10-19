package vt

import (
	"net/http"

	"saldo/pkg/db"

	"github.com/vmkteam/embedlog"
	zm "github.com/vmkteam/zenrpc-middleware"
	"github.com/vmkteam/zenrpc/v2"
)

var (
	ErrUnauthorized   = httpAsRPCError(http.StatusUnauthorized)
	ErrForbidden      = httpAsRPCError(http.StatusForbidden)
	ErrNotFound       = httpAsRPCError(http.StatusNotFound)
	ErrInternal       = httpAsRPCError(http.StatusInternalServerError)
	ErrNotImplemented = httpAsRPCError(http.StatusNotImplemented)
)

var allowDebugFn = func() zm.AllowDebugFunc {
	return func(req *http.Request) bool {
		return req != nil && req.FormValue("__level") == "5"
	}
}

func httpAsRPCError(code int) *zenrpc.Error {
	return zenrpc.NewStringError(code, http.StatusText(code))
}

// New returns new zenrpc Server for metrics and monitoring.
func New(dbo db.DB, logger embedlog.Logger, isDevel bool) zenrpc.Server {
	rpc := zenrpc.NewServer(zenrpc.Options{
		ExposeSMD: true,
		AllowCORS: false,
	})

	// middleware
	rpc.Use(
		zm.WithHeaders(),
		zm.WithDevel(isDevel),
		zm.WithNoCancelContext(),
		zm.WithMetrics("vt"),
		zm.WithSLog(logger.Print, zm.DefaultServerName, nil),
		zm.WithErrorSLog(logger.Error, zm.DefaultServerName, nil),
		zm.WithSQLLogger(dbo.DB, isDevel, allowDebugFn(), allowDebugFn()),
		zm.WithTiming(isDevel, allowDebugFn()),
		zm.WithSentry(zm.DefaultServerName),
	)

	// No services registered - this RPC server is kept for metrics only
	// Services will be added later when needed for Telegram bot

	return rpc
}
