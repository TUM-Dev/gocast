package dao

import (
	"fmt"
	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	"github.com/hashicorp/go-multierror"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// sentryGinInitMiddleware starts a sub span if a request already has a running SentryGin span.
func sentryGinInitMiddleware(db *gorm.DB) {
	if ctx, ok := db.Statement.Context.(*gin.Context); ok {
		if span, ok := ctx.Get("sentry.span"); ok {
			sqlSpan := sentry.StartSpan(span.(*sentry.Span).Context(), "SQL")
			ctx.Set("sentry.sqlSpan", sqlSpan)
		}
	}
}

// sentryGinAfterMiddleware finishes a sub span if a request already has a running sentry.sqlSpan span.
func sentryGinAfterMiddleware(db *gorm.DB) {
	if ctx, ok := db.Statement.Context.(*gin.Context); ok {
		if span, ok := ctx.Get("sentry.sqlSpan"); ok {
			sqlSpan := span.(*sentry.Span)
			sqlSpan.SetTag("query", db.Statement.SQL.String())
			sqlSpan.SetTag("rows_affected", fmt.Sprintf("%d", db.RowsAffected))
			sqlSpan.Finish()
		}
	}
}

func InitSentryMiddlewares() {
	var callbacksErrs error
	if err := DB.Callback().Query().Before("*").Register("beforeSentry", sentryGinInitMiddleware); err != nil {
		callbacksErrs = multierror.Append(callbacksErrs, err)
	}
	if err := DB.Callback().Query().After("*").Register("afterSentry", sentryGinAfterMiddleware); err != nil {
		callbacksErrs = multierror.Append(callbacksErrs, err)
	}
	if callbacksErrs != nil {
		log.WithError(callbacksErrs).Fatal("Error registering db callbacks")
	}
}
