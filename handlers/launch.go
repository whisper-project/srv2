/*
 * Copyright 2024-2026 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package handlers

import (
	"fmt"
	"net/http"
	"os"
	"strconv"

	"github.com/whisper-project/server.golang/middleware"
	platform2 "github.com/whisper-project/server.golang/platform"
	"github.com/whisper-project/server.golang/storage"

	"gopkg.in/gomail.v2"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func PostLaunchHandler(c *gin.Context) {
	clientType := c.GetHeader("X-Client-Type")
	clientId := c.GetHeader("X-Client-Id")
	profileId := c.GetHeader("X-Profile-Id")
	var emailHash string
	var err error
	if err = c.ShouldBindJSON(&emailHash); err != nil || emailHash == "" {
		middleware.CtxLog(c).Info("Invalid email hash",
			zap.String("hash", emailHash), zap.Error(err))
		c.JSON(400, gin.H{"error": "Invalid request"})
		return
	}
	if profileId == "" {
		// The client doesn't know their profile; they are requesting it.
		// So look for an existing profile that matches the email hash that was sent.
		profileId, _ = storage.EmailProfile(emailHash)
		if profileId != "" {
			// there's an existing profile, so the user needs to authenticate.
			middleware.CtxLog(c).Info("Profile exists, need authentication",
				zap.String("profileId", profileId))
			c.Writer.Header().Set("WWW-Authenticate", fmt.Sprintf(`Bearer realm="%s"`, profileId))
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Provide authorization token"})
			return
		}
		// there's no existing profile, so create one and return it
		p, err := storage.NewLaunchProfile(clientType, emailHash, clientId)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		middleware.CtxLog(c).Info("Created new profile",
			zap.String("email", p.EmailHash), zap.String("profileId", p.Id), zap.String("clientId", clientId))
		response := map[string]string{"id": p.Id, "name": p.Name, "secret": p.Secret}
		c.JSON(http.StatusCreated, response)
		return
	}
	// make sure the client-supplied profile ID is real before responding
	emailProfileId, err := storage.EmailProfile(emailHash)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if emailProfileId != profileId {
		middleware.CtxLog(c).Info("Profile for email doesn't match client-supplied profile",
			zap.String("email profileId", emailProfileId), zap.String("client profileId", profileId))
		c.JSON(http.StatusNotFound, gin.H{"error": "The supplied profile ID doesn't match the supplied email"})
		return
	}
	// Check the user's authentication
	p := AuthenticateRequest(c)
	if p == nil {
		return
	}
	// they are authenticated, so remember them against this client
	storage.ObserveClientLaunch(clientType, clientId, profileId)
	middleware.CtxLog(c).Info("Authenticated profile",
		zap.String("profileId", p.Id), zap.String("clientId", clientId),
		zap.String("name", p.Name), zap.String("email", p.EmailHash))
	c.JSON(http.StatusOK, gin.H{"name": p.Name})
	return
}

func PostRequestEmailHandler(c *gin.Context) {
	var email string
	err := c.Bind(&email)
	if err != nil || email == "" {
		middleware.CtxLog(c).Error("Invalid request for email", zap.String("email", email), zap.Error(err))
		c.JSON(400, gin.H{"error": "Invalid request"})
		return
	}
	hash := platform2.MakeSha1(email)
	// look for a profile that matches the email
	ctx := c.Request.Context()
	profileId, err := platform2.MapGet(ctx, storage.EmailProfileMap, hash)
	if err != nil {
		middleware.CtxLog(c).Error("Map failure", zap.String("email", hash), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// if we don't find one, let the client know
	if profileId == "" {
		middleware.CtxLog(c).Error("No profile found for email", zap.String("email", hash))
		c.JSON(http.StatusNotFound, gin.H{"error": "No profile found for email"})
		return
	}
	// otherwise, load the profile, and send email with password
	p := &storage.Profile{Id: profileId}
	if err := platform2.LoadFields(ctx, p); err != nil {
		middleware.CtxLog(c).Error("Load Fields failure", zap.String("profileId", profileId), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
	middleware.CtxLog(c).Info("Sending email", zap.String("profileId", p.Id), zap.String("password", p.Secret))
	if err := sendMail(email, p.Secret); err != nil {
		middleware.CtxLog(c).Error("Send email failure", zap.String("profileId", p.Id), zap.Error(err))
	}
	c.Status(http.StatusNoContent)
}

func GetShutdownHandler(c *gin.Context) {
	if AuthenticateRequest(c) != nil {
		return
	}
	clientId := c.GetHeader("X-Client-Id")
	storage.ObserveClientActivity(clientId, "shutdown")
	c.Status(http.StatusNoContent)
}

func sendMail(to, pw string) error {
	m := gomail.NewMessage()
	m.SetHeader("From", "no-reply@whisper-project.com")
	m.SetHeader("To", to)
	m.SetHeader("Subject", "Your Whisper profile information")
	m.SetBody("text/html", "As requested, your Whisper profile password is: <pre>"+pw+"</pre>")

	account := os.Getenv("SMTP_ACCOUNT")
	password := os.Getenv("SMTP_PASSWORD")
	host := os.Getenv("SMTP_HOST")
	port, err := strconv.ParseInt(os.Getenv("SMTP_PORT"), 10, 16)
	if err != nil || account == "" || password == "" || host == "" {
		return fmt.Errorf("missing SMTP environment variables")
	}
	d := gomail.NewDialer(host, int(port), account, password)

	if err := d.DialAndSend(m); err != nil {
		return err
	}
	return nil
}
