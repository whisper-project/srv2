/*
 * Copyright 2024-2026 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package platform

import (
	"fmt"

	"github.com/redis/go-redis/v9"
)

var (
	clientUrl string
	client    *redis.Client
	keyPrefix string
)

func GetDb() (*redis.Client, string) {
	config := GetConfig()
	if client != nil && clientUrl == config.DbUrl && keyPrefix == config.DbProjectPrefix+config.DbEnvPrefix {
		return client, keyPrefix
	}
	clientUrl = config.DbUrl
	client = getClientFromUrl(clientUrl)
	keyPrefix = config.DbProjectPrefix + config.DbEnvPrefix
	return client, keyPrefix
}

func GetLegacyDb() (*redis.Client, string) {
	config := GetConfig()
	clientUrl = config.DbUrl
	client = getClientFromUrl(clientUrl)
	keyPrefix = config.LegacyDbProjectPrefix + config.LegacyDbEnvPrefix
	return client, keyPrefix
}

func getClientFromUrl(url string) *redis.Client {
	opts, err := redis.ParseURL(url)
	if err != nil {
		panic(fmt.Sprintf("invalid Redis url: %v", err))
	}
	return redis.NewClient(opts)
}
