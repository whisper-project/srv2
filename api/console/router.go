/*
 * Copyright 2024-2026 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package console

import (
	"github.com/gin-gonic/gin"
	handlers2 "github.com/whisper-project/srv2/handlers"
)

func AddRoutes(r *gin.RouterGroup) {
	r.POST("/launch", handlers2.PostLaunchHandler)
	r.GET("/shutdown", handlers2.GetShutdownHandler)
	r.PATCH("/profile", handlers2.PatchProfileHandler)
	r.POST("/request-email", handlers2.PostRequestEmailHandler)
	r.GET("/whisper-conversations", handlers2.GetProfileWhisperConversationsHandler)
	r.POST("/whisper-conversations", handlers2.PostProfileWhisperConversationHandler)
	r.GET("/whisper-conversations/:name", handlers2.GetProfileWhisperConversationIdHandler)
	r.DELETE("/whisper-conversations/:name", handlers2.DeleteProfileWhisperConversationHandler)
}
