/*
 * Copyright 2024-2026 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/whisper-project/server.golang/middleware"
	"github.com/whisper-project/server.golang/platform"
	"github.com/whisper-project/server.golang/storage"
	"go.uber.org/zap"
)

func IsAllowedListener(c *gin.Context, conversationId, profileId string) (bool, error) {
	m := storage.AllowedListeners(conversationId)
	ok, err := platform.IsSetMember(c.Request.Context(), m, profileId)
	if err != nil {
		middleware.CtxLog(c).Error("storage failure checking allowed listener",
			zap.String("conversationId", conversationId), zap.String("profileId", profileId), zap.Error(err))
		return false, err
	}
	return ok, nil
}

func AddAllowedListener(c *gin.Context, conversationId, profileId string) error {
	m := storage.AllowedListeners(conversationId)
	err := platform.AddSetMembers(c.Request.Context(), m, profileId)
	if err != nil {
		middleware.CtxLog(c).Error("storage failure adding allowed listener",
			zap.String("conversationId", conversationId), zap.String("profileId", profileId), zap.Error(err))
		return err
	}
	return nil
}

func RemoveAllowedListener(c *gin.Context, conversationId, profileId string) error {
	m := storage.AllowedListeners(conversationId)
	err := platform.RemoveSetMembers(c.Request.Context(), m, profileId)
	if err != nil {
		middleware.CtxLog(c).Error("storage failure removing allowed listener",
			zap.String("conversationId", conversationId), zap.String("profileId", profileId), zap.Error(err))
	}
	return nil
}
