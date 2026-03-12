/*
 * Copyright 2024-2026 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/whisper-project/srv2/middleware"
	"github.com/whisper-project/srv2/platform"
	"github.com/whisper-project/srv2/storage"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

func AuthenticateRequest(c *gin.Context) *storage.Profile {
	clientId := c.GetHeader("X-Client-Id")
	profileId := c.GetHeader("X-Profile-Id")
	if clientId == "" || profileId == "" {
		middleware.CtxLog(c).Info("missing identification header",
			zap.String("X-Client-Id", clientId), zap.String("X-Profile-Id", profileId))
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing identification header"})
		return nil
	}
	authToken := c.GetHeader("Authorization")
	if authToken == "" {
		middleware.CtxLog(c).Info("Profile exists, need authorization", zap.String("profileId", profileId))
		c.Writer.Header().Set("WWW-Authenticate", fmt.Sprintf(`Bearer realm="%s"`, profileId))
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Provide an authorization token"})
		return nil
	} else if len(authToken) > len("Bearer ") {
		authToken = authToken[len("Bearer "):]
	} else {
		middleware.CtxLog(c).Info("invalid Authorization header", zap.String("header", c.GetHeader("Authorization")))
		c.JSON(http.StatusForbidden, gin.H{"error": "invalid bearer token"})
		return nil
	}
	p, err := storage.GetProfile(profileId)
	if err != nil {
		if errors.Is(err, platform.NotFoundError) {
			c.JSON(http.StatusForbidden, gin.H{"error": "profile not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database failure"})
		}
		return nil
	}
	if authenticateJwt(c, authToken, clientId, profileId, p.Secret) {
		return p
	}
	return nil
}

func authenticateJwt(c *gin.Context, bearerToken, clientId, profileId, secret string) bool {
	key, err := uuid.Parse(secret)
	if err != nil {
		middleware.CtxLog(c).Error("Secret is not a UUID",
			zap.String("profileId", profileId), zap.String("secret", secret), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "server data is corrupt - please report a bug!"})
		return false
	}
	keyBytes, err := key.MarshalBinary()
	if err != nil {
		middleware.CtxLog(c).Error("Marshal UUID failure",
			zap.String("profileId", profileId), zap.String("secret", secret), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "server behavior is corrupt - please report a bug!"})
		return false
	}
	validator := func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			// notest
			return false, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return keyBytes, nil
	}
	token, err := jwt.Parse(bearerToken, validator, jwt.WithValidMethods([]string{"HS256", "HS384", "HS512"}))
	if err != nil {
		middleware.CtxLog(c).Info("Invalid bearer token", zap.String("profileId", profileId), zap.Error(err))
		c.JSON(http.StatusForbidden, gin.H{"error": "invalid bearer token"})
		return false
	}
	if issuer, err := token.Claims.GetIssuer(); err != nil {
		middleware.CtxLog(c).Info("Invalid issuer claim", zap.Error(err))
		c.JSON(http.StatusForbidden, gin.H{"error": "invalid bearer token"})
		return false
	} else if issuer != clientId {
		middleware.CtxLog(c).Info("Token issuer doesn't match client id",
			zap.String("clientId", clientId), zap.String("issuer", issuer))
		c.JSON(http.StatusForbidden, gin.H{"error": "invalid bearer token"})
		return false
	}
	if subject, err := token.Claims.GetSubject(); err != nil {
		middleware.CtxLog(c).Info("Invalid subject claim", zap.Error(err))
		c.JSON(http.StatusForbidden, gin.H{"error": "invalid bearer token"})
		return false
	} else if subject != profileId {
		middleware.CtxLog(c).Info("Token subject doesn't match profile id",
			zap.String("profileId", profileId), zap.String("subject", subject))
		c.JSON(http.StatusForbidden, gin.H{"error": "invalid bearer token"})
		return false
	}
	if issuedAt, err := token.Claims.GetIssuedAt(); err != nil {
		middleware.CtxLog(c).Info("Invalid issuedAt claim", zap.Error(err))
		c.JSON(http.StatusForbidden, gin.H{"error": "invalid bearer token"})
		return false
	} else if age := time.Now().Unix() - issuedAt.Unix(); (age < -300) || (age > 300) {
		middleware.CtxLog(c).Info("Token age is too far off",
			zap.String("issuedAt", issuedAt.String()), zap.Int64("age", age))
		c.JSON(http.StatusForbidden, gin.H{"error": "invalid bearer token"})
		return false
	}
	return true
}
