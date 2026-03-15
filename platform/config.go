/*
 * Copyright 2024-2026 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package platform

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/dotenv-org/godotenvvault"
)

type Environment struct {
	AblyPublishKey   string
	AblySubscribeKey string
	AgePublicKey     string
	AgeSecretKey     string
	AwsAccessKey     string
	AwsBucket        string
	AwsRegion        string
	AwsRootPath      string
	AwsSecretKey     string
	DbKeyPrefix      string
	DbUrl            string
	HttpHost         string
	HttpPort         int
	HttpScheme       string
	Name             string
	SmtpCredId       string
	SmtpCredSecret   string
	SmtpHost         string
	SmtpPort         int
}

//goland:noinspection SpellCheckingInspection
var (
	loadedConfig = Environment{
		AblyPublishKey:   "",
		AblySubscribeKey: "",
		AgePublicKey:     "",
		AgeSecretKey:     "",
		DbKeyPrefix:      "c:",
		DbUrl:            "redis://",
		HttpHost:         "localhost",
		HttpPort:         8080,
		HttpScheme:       "http",
		Name:             "CI",
		SmtpCredId:       "",
		SmtpCredSecret:   "",
		SmtpHost:         "",
		SmtpPort:         2500,
	}
)

func init() {
	_ = SetConfig("d")
}

// GetConfig returns the current Environment.
//
// There is always a current environment. When this module is first loaded,
// it attempts to load a development environment via `SetConfig("d")`.
// If that fails, it falls back to a `CI` environment that has no
// secret values in it.
func GetConfig() Environment {
	return loadedConfig
}

// SetConfig sets the environment based on the dotenv file of the specified name.
//
// If you don't specify any name, you get the environment from the current directory's
// `.env.vault` file selected by the `DOTENV_KEY` environment variable. Only the
// current directory is searched for the `.env.vault` file (or fallback `.env` file).
//
// If you do specify a name, you are specifying that you want to the environment
// loaded from a `.env*` file. Only the first character of the name matters,
// and it must be one of 'd' (for `.env`), 's' for `.env.staging`,
// 'p' for `.env.production`, or 't' for `.env.testing`. And the file is looked
// for not only in the current directory, but in 4 levels of parent directory.
func SetConfig(name string) error {
	// notest
	if name == "" {
		return setEnvConfig("")
	}
	if strings.HasPrefix(name, "d") {
		return setEnvConfig(".env")
	}
	if strings.HasPrefix(name, "s") {
		return setEnvConfig(".env.staging")
	}
	if strings.HasPrefix(name, "p") {
		return setEnvConfig(".env.production")
	}
	if strings.HasPrefix(name, "t") {
		return setEnvConfig(".env.testing")
	}
	return fmt.Errorf("unknown environment: %s", name)
}

func setEnvConfig(filename string) error {
	var d string
	var err error
	if filename == "" {
		err = godotenvvault.Overload()
	} else {
		if d, err = FindEnvFile(filename); err == nil {
			err = godotenvvault.Overload(d + filename)
		}
	}
	if err != nil {
		return fmt.Errorf("error loading environment: %w", err)
	}
	getEnvPort := func(s string, d int) int {
		val, _ := strconv.Atoi(s)
		if val <= 0 {
			// notest
			return d
		}
		return val
	}
	loadedConfig = Environment{
		AblyPublishKey:   os.Getenv("ABLY_PUBLISH_KEY"),
		AblySubscribeKey: os.Getenv("ABLY_SUBSCRIBE_KEY"),
		AgePublicKey:     os.Getenv("AGE_PUBLIC_KEY"),
		AgeSecretKey:     os.Getenv("AGE_SECRET_KEY"),
		AwsAccessKey:     os.Getenv("AWS_ACCESS_KEY"),
		AwsBucket:        os.Getenv("AWS_BUCKET"),
		AwsRegion:        os.Getenv("AWS_REGION"),
		AwsRootPath:      os.Getenv("AWS_FOLDER_PATH"),
		AwsSecretKey:     os.Getenv("AWS_SECRET_KEY"),
		DbKeyPrefix:      os.Getenv("DB_KEY_PREFIX"),
		DbUrl:            os.Getenv("REDIS_URL"),
		HttpHost:         os.Getenv("HTTP_HOST"),
		HttpPort:         getEnvPort(os.Getenv("HTTP_PORT"), 8080),
		HttpScheme:       os.Getenv("HTTP_SCHEME"),
		Name:             os.Getenv("ENVIRONMENT_NAME"),
		SmtpCredId:       os.Getenv("SMTP_CRED_ID"),
		SmtpCredSecret:   os.Getenv("SMTP_CRED_SECRET"),
		SmtpHost:         os.Getenv("SMTP_HOST"),
		SmtpPort:         getEnvPort(os.Getenv("SMTP_PORT"), 2025),
	}
	return nil
}

func FindEnvFile(name string) (string, error) {
	for i := range 5 {
		d := ""
		for range i {
			d += "../"
		}
		if _, err := os.Stat(d + name); err == nil {
			return d, nil
		}
	}
	return "", fmt.Errorf("no file %q found in current directory or 4 levels of parent", name)
}
