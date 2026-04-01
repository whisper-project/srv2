/*
 * Copyright 2024-2026 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package platform

import (
	"log"
	"os"
	"strconv"

	"github.com/dotenv-org/godotenvvault"
)

var (
	// the global environment
	loadedConfig = Environment{}
	// known environment names and their matching dotenv filenames
	knownEnvironments = map[string]string{
		"development": ".env",
		"dev":         ".env",
		"d":           ".env",
		"production":  ".env.production",
		"prod":        ".env.production",
		"p":           ".env.production",
		"staging":     ".env.staging",
		"stage":       ".env.staging",
		"s":           ".env.staging",
		"testing":     ".env.testing",
		"test":        ".env.testing",
		"t":           ".env.testing",
	}
)

type Environment struct {
	AblyPublishKey        string
	AblySubscribeKey      string
	AgePublicKey          string
	AgeSecretKey          string
	AwsAccessKey          string
	AwsBucket             string
	AwsRegion             string
	AwsRootPath           string
	AwsSecretKey          string
	DbEnvPrefix           string
	DbProjectPrefix       string
	DbUrl                 string
	ElevenLabsApiKey      string
	HttpHost              string
	HttpPort              int
	HttpScheme            string
	LegacyDbEnvPrefix     string
	LegacyDbProjectPrefix string
	LegacyDbUrl           string
	Name                  string
	ResembleToken         string
	SmtpCredId            string
	SmtpCredSecret        string
	SmtpHost              string
	SmtpPort              int
}

func (env *Environment) loadEnvConfig(envNames ...string) error {
	filenames := findEnvFiles(envNames...)
	if err := godotenvvault.Overload(filenames...); err != nil {
		return err
	}
	getEnvPort := func(s string, d int) int {
		val, _ := strconv.Atoi(s)
		if val <= 0 {
			// notest
			return d
		}
		return val
	}
	*env = Environment{
		AblyPublishKey:        os.Getenv("ABLY_PUBLISH_KEY"),
		AblySubscribeKey:      os.Getenv("ABLY_SUBSCRIBE_KEY"),
		AgePublicKey:          os.Getenv("AGE_PUBLIC_KEY"),
		AgeSecretKey:          os.Getenv("AGE_SECRET_KEY"),
		AwsAccessKey:          os.Getenv("AWS_ACCESS_KEY"),
		AwsBucket:             os.Getenv("AWS_BUCKET"),
		AwsRegion:             os.Getenv("AWS_REGION"),
		AwsRootPath:           os.Getenv("AWS_FOLDER_PATH"),
		AwsSecretKey:          os.Getenv("AWS_SECRET_KEY"),
		DbEnvPrefix:           os.Getenv("DB_ENV_PREFIX"),
		DbProjectPrefix:       os.Getenv("DB_PROJECT_PREFIX"),
		DbUrl:                 os.Getenv("DB_URL"),
		ElevenLabsApiKey:      os.Getenv("ELEVENLABS_API_KEY"),
		HttpHost:              os.Getenv("HTTP_HOST"),
		HttpPort:              getEnvPort(os.Getenv("HTTP_PORT"), 8080),
		HttpScheme:            os.Getenv("HTTP_SCHEME"),
		LegacyDbEnvPrefix:     os.Getenv("LEGACY_DB_ENV_PREFIX"),
		LegacyDbProjectPrefix: os.Getenv("LEGACY_DB_PROJECT_PREFIX"),
		LegacyDbUrl:           os.Getenv("LEGACY_DB_URL"),
		Name:                  os.Getenv("ENVIRONMENT_NAME"),
		ResembleToken:         os.Getenv("RESEMBLE_TOKEN"),
		SmtpCredId:            os.Getenv("SMTP_CRED_ID"),
		SmtpCredSecret:        os.Getenv("SMTP_CRED_SECRET"),
		SmtpHost:              os.Getenv("SMTP_HOST"),
		SmtpPort:              getEnvPort(os.Getenv("SMTP_PORT"), 2025),
	}
	return nil
}

// findEnvFiles returns the relative paths to the environment files
// for the given environment names. It looks for the files in the
// current directory, then in up to five levels of parent. It returns
// all the ones it can find.
func findEnvFiles(names ...string) []string {
	var filenames []string
	for _, name := range names {
		filename, ok := knownEnvironments[name]
		if !ok {
			continue
		}
		for d, j := "", 0; j < 5; d, j = "../"+d, j+1 {
			if _, err := os.Stat(d + filename); err == nil {
				filenames = append(filenames, d+filename)
				break
			}
		}
	}
	return filenames
}

// GetConfig returns a pointer to the current Environment.
//
// The use of a pointer allows the Environment to be altered.
// This is typically done for testing purposes. Any alterations
// will be overwritten at the next SetConfig call.
//
// There is always a current environment. When this module is first loaded,
// it loads the vault environment and falls back to the development environment.
// If that fails, it panics.
func GetConfig() *Environment {
	return &loadedConfig
}

// SetConfig tries to load the vault environment. If that fails,
// it falls back to loading the passed environments, using the
// first one that succeeds.
func SetConfig(names ...string) error {
	return loadedConfig.loadEnvConfig(names...)
}

func init() {
	// start with the vault environment, falling back to the development environment
	if err := SetConfig("development"); err != nil {
		log.Panicf("Cannot run without a valid environment: %v", err)
	}
}
