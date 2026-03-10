/*
 * Copyright 2024-2026 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package console

import (
	"github.com/gin-gonic/gin"
	"github.com/whisper-project/server.golang/handlers"
)

func AddRoutes(r *gin.RouterGroup) {
	r.POST("/launch", handlers.PostLaunchHandler)
	r.GET("/shutdown", handlers.GetShutdownHandler)
	r.PATCH("/profile", handlers.PatchProfileHandler)
	r.POST("/request-email", handlers.PostRequestEmailHandler)
	r.GET("/whisper-conversations", handlers.GetProfileWhisperConversationsHandler)
	r.POST("/whisper-conversations", handlers.PostProfileWhisperConversationHandler)
	r.GET("/whisper-conversations/:name", handlers.GetProfileWhisperConversationIdHandler)
	r.DELETE("/whisper-conversations/:name", handlers.DeleteProfileWhisperConversationHandler)
	r.GET("/whisper-start/:conversationId", handlers.StartWhisperSessionHandler)
	r.GET("/listen-start/:conversationId", handlers.StartListenSessionHandler)
	r.GET("/authenticate-conversation/:conversationId", handlers.GetClientSessionTokenHandler)
}
