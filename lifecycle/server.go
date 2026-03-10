/*
 * Copyright 2024-2026 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package lifecycle

import (
	"context"
	"errors"
	"net/http"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/whisper-project/server.golang/middleware"
	"github.com/whisper-project/server.golang/platform"
	"github.com/whisper-project/server.golang/storage"
)

// Startup takes a configured router and runs a server instance with it as handler.
// The instance is configured so that it can be exited cleanly,
// and it resumes any sessions left by the last instance.
//
// This code based on [this example](https://github.com/gin-gonic/examples/blob/master/graceful-shutdown/graceful-shutdown/notify-with-context/server.go)
func Startup(router *gin.Engine, hostPort string) {
	// Create context that listens for the interrupt signal from the OS.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Resume listening to suspended conversations left from the last server instance
	go StartAllSuspendedSessions()

	// Run the server in a goroutine so that this instance survives it
	running := true
	srv := &http.Server{Addr: hostPort, Handler: router}
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			sLog().Error("server crashed", zap.Error(err))
		}
		running = false
	}()

	// Listen for the interrupt signal, and restore default behavior
	<-ctx.Done()
	stop()
	sLog().Info("interrupt received")

	// Shutdown all the ongoing conversations and the http server instance.
	// If the server is still running, we give it until we have suspended
	// all the current conversations before to finish handling open requests,
	// at which point we force quit it.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if running {
		go func() {
			if err := srv.Shutdown(ctx); err != nil {
				sLog().Error("http server terminated with error", zap.Error(err))
			}
			sLog().Info("http server gracefully stopped")
		}()
	}
	Shutdown(false)
	cancel()
	sLog().Info("shutdown completed cleanly")
}

// Shutdown cleanly terminates this server instance.
//
// If [withoutSuspend] is specified, all sessions in progress are
// terminated instead of being suspended for use by another session
func Shutdown(withoutSuspend bool) {
	if withoutSuspend {
		sLog().Info("terminating all sessions")
		count := EndAllSessions()
		sLog().Info("terminated all sessions", zap.Int("session count", count))
		return
	}
	sLog().Info("suspending all sessions")
	notify := make(chan int)
	go ShutdownAllSessions(notify)
	count := <-notify
	sLog().Info("suspended all sessions", zap.Int("session count", count))
}

func CreateEngine() (*gin.Engine, error) {
	var logger *zap.Logger
	var err error
	if platform.GetConfig().Name == "production" {
		gin.SetMode(gin.ReleaseMode)
		logger, err = zap.NewProduction()
	} else {
		logger, err = zap.NewDevelopment()
	}
	if err != nil {
		return nil, err
	}
	defer logger.Sync()
	engine := middleware.CreateCoreEngine(logger)
	err = engine.SetTrustedProxies(nil)
	if err != nil {
		return nil, err
	}
	storage.ServerLogger = logger
	return engine, nil
}

func sLog() *zap.Logger {
	return storage.ServerLogger
}
