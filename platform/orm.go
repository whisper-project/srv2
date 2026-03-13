/*
 * Copyright 2024-2026 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package platform

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// A RedisKey object computes a Redis storage key.
type RedisKey interface {
	StoragePrefix() string
	StorageId() string
}

// RedisKeyTester validates the methods on a RedisKey type.
// Hand it a value of the type, the expected prefix of the type, and the expected ID of the value.
func RedisKeyTester[K RedisKey](t *testing.T, rk K, prefix, id string) {
	t.Helper()
	// if this is a pointer type, check for handling nil values
	if reflect.ValueOf(rk).Kind() == reflect.Ptr {
		var nk K
		if nk.StorageId() != "" {
			t.Errorf("expecting empty storage id, got %v", nk.StorageId())
		}
	}
	if rk.StoragePrefix() != prefix {
		t.Errorf("(%T).StoragePrefix() returned %q, expected %q", rk, rk.StoragePrefix(), prefix)
	}
	if v := rk.StorageId(); v != id {
		t.Errorf("(%T).StorageId() returned %q. expected %q", rk, v, id)
	}
}

// A RedisValue object knows how to map to and from stored Redis values.
type RedisValue interface {
	ToRedis() ([]byte, error)
	FromRedis([]byte) error
}

// RedisValueTester validates the methods on a RedisValue type.
// Hand it a concrete value of the type, a second one with a different value,
// and a comparator function for the two values (which will end up the same).
func RedisValueTester[V RedisValue](t *testing.T, v1, v2 V, cmp func(V, V) bool) {
	t.Helper()
	if reflect.ValueOf(v1).Kind() != reflect.Ptr {
		t.Fatalf("RedisValue methods must have pointer receivers; {%T} doesn't", reflect.ValueOf(v1))
	}
	if reflect.ValueOf(v2).Kind() != reflect.Ptr {
		t.Fatalf("RedisValue methods must have pointer receivers; {%T} doesn't", reflect.ValueOf(v2))
	}
	if cmp(v1, v2) {
		t.Fatalf("values must differ to begin with (%v == %v)", v1, v2)
	}
	b, err := v1.ToRedis()
	if err != nil {
		t.Fatalf("Serialization failed: %v", err)
	}
	err = v2.FromRedis(b)
	if err != nil {
		t.Fatalf("Deserialization failed: %v", err)
	}
	if !cmp(v1, v2) {
		t.Fatalf("values must agree at the end (%v != %v)", v1, v2)
	}
}

// SetExpiration sets the TTL of the given RedisKey rk to the given secs.
//
// It is an error to pass in a negative number of seconds.
func SetExpiration[K RedisKey](ctx context.Context, rk K, secs int64) error {
	db, prefix := GetDb()
	key := prefix + rk.StoragePrefix() + rk.StorageId()
	res := db.Expire(ctx, key, time.Duration(secs)*time.Second)
	if err := res.Err(); err != nil {
		return err
	}
	return nil
}

// SetExpirationAt sets the expiration date of the given RedisKey rk to the given etime.
//
// It is an error to set the expiration date to a time in the past.
func SetExpirationAt[K RedisKey](ctx context.Context, rk K, etime time.Time) error {
	db, prefix := GetDb()
	key := prefix + rk.StoragePrefix() + rk.StorageId()
	res := db.ExpireAt(ctx, key, etime)
	if err := res.Err(); err != nil {
		return err
	}
	return nil
}

// DeleteStorage removes any storage for the given RedisKey rk from the database.
//
// Deleting a non-stored object is a no-op.
func DeleteStorage[K RedisKey](ctx context.Context, rk K) error {
	if rk.StorageId() == "" {
		return fmt.Errorf("storable has no ID")
	}
	db, prefix := GetDb()
	key := prefix + rk.StoragePrefix() + rk.StorageId()
	res := db.Del(ctx, key)
	if err := res.Err(); err != nil {
		return err
	}
	return nil
}

// FetchString returns the string value of a RedisKey rk from the database.
//
// If the key has no database storage, an empty string value is returned.
func FetchString[K RedisKey](ctx context.Context, rk K) (string, error) {
	db, prefix := GetDb()
	key := prefix + rk.StoragePrefix() + rk.StorageId()
	res := db.Get(ctx, key)
	if err := res.Err(); err != nil {
		if errors.Is(err, redis.Nil) {
			return "", nil
		}
		return "", err
	}
	return res.Val(), nil
}

// StoreString stores the string value of a RedisKey rk to the database.
func StoreString[K RedisKey](ctx context.Context, rk K, val string) error {
	db, prefix := GetDb()
	key := prefix + rk.StoragePrefix() + rk.StorageId()
	res := db.Set(ctx, key, val, 0)
	if err := res.Err(); err != nil {
		return err
	}
	return nil
}

// FetchSetMembers returns the "set of strings" value of the given RedisKey rk from the database.
//
// If the key has no database storage, an empty slice is returned.
func FetchSetMembers[K RedisKey](ctx context.Context, rk K) ([]string, error) {
	db, prefix := GetDb()
	key := prefix + rk.StoragePrefix() + rk.StorageId()
	res := db.SMembers(ctx, key)
	if err := res.Err(); err != nil {
		return nil, err
	}
	return res.Val(), nil
}

// IsSetMember checks whether the given member is in the "set of strings" value of the given RedisKey rk.
func IsSetMember[K RedisKey](ctx context.Context, rk K, member string) (bool, error) {
	db, prefix := GetDb()
	key := prefix + rk.StoragePrefix() + rk.StorageId()
	res := db.SIsMember(ctx, key, member)
	if err := res.Err(); err != nil {
		return false, err
	}
	return res.Val(), nil
}

// AddSetMembers adds the given members to the "set of strings" value of the given RedisKey rk.
func AddSetMembers[K RedisKey](ctx context.Context, rk K, members ...string) error {
	if len(members) == 0 {
		// nothing to add
		return nil
	}
	db, prefix := GetDb()
	key := prefix + rk.StoragePrefix() + rk.StorageId()
	args := argsAsAny(members...)
	res := db.SAdd(ctx, key, args...)
	if err := res.Err(); err != nil {
		return err
	}
	return nil
}

// RemoveSetMembers removes the given members from the "set of strings" value of the given RedisKey rk.
//
// Removing non-present members is a no-op.
func RemoveSetMembers[K RedisKey](ctx context.Context, rk K, members ...string) error {
	if len(members) == 0 {
		// nothing to delete
		return nil
	}
	db, prefix := GetDb()
	key := prefix + rk.StoragePrefix() + rk.StorageId()
	args := argsAsAny(members...)
	res := db.SRem(ctx, key, args...)
	if err := res.Err(); err != nil {
		return err
	}
	return nil
}

func argsAsAny(args ...string) []any {
	asAny := make([]any, len(args))
	for i, arg := range args {
		asAny[i] = any(arg)
	}
	return asAny
}

// Scored Sets

// FetchSsRangeByIndex returns the slice of string members delimited by the given start and end indices
// from the "sorted set of strings" value of the RedisKey rk
func FetchSsRangeByIndex[K RedisKey](ctx context.Context, rk K, start, end int64) ([]string, error) {
	db, prefix := GetDb()
	key := prefix + rk.StoragePrefix() + rk.StorageId()
	res := db.ZRange(ctx, key, start, end)
	if err := res.Err(); err != nil {
		return nil, err
	}
	return res.Val(), nil
}

// FetchSsRangeByScore returns the slice of string members delimited by the given min and max values
// from the "sorted set of strings" value of the RedisKey rk.
func FetchSsRangeByScore[K RedisKey](ctx context.Context, rk K, min, max float64) ([]string, error) {
	db, prefix := GetDb()
	key := prefix + rk.StoragePrefix() + rk.StorageId()
	args := redis.ZRangeArgs{
		Key:     key,
		Start:   min,
		Stop:    max,
		ByScore: true,
	}
	res := db.ZRangeArgs(ctx, args)
	if err := res.Err(); err != nil {
		return nil, err
	}
	return res.Val(), nil
}

// AddSsMember adds the given member with the given score
// to the "sorted set of strings" value of the RedisKey rk.
func AddSsMember[K RedisKey](ctx context.Context, rk K, score float64, member string) error {
	db, prefix := GetDb()
	key := prefix + rk.StoragePrefix() + rk.StorageId()
	res := db.ZAdd(ctx, key, redis.Z{Score: score, Member: member})
	if err := res.Err(); err != nil {
		return err
	}
	return nil
}

// RemoveSsMember removes the given member
// from the "sorted set of strings" value of the RedisKey rk.
func RemoveSsMember[K RedisKey](ctx context.Context, rk K, member string) error {
	db, prefix := GetDb()
	key := prefix + rk.StoragePrefix() + rk.StorageId()
	res := db.ZRem(ctx, key, member)
	if err := res.Err(); err != nil {
		return err
	}
	return nil
}

// GetSsMemberScore returns the score of the given member
// in the "sorted set of strings" value of the RedisKey rk.
//
// If the member isn't actually a member of the set, 0 is returned.
func GetSsMemberScore[K RedisKey](ctx context.Context, rk K, member string) (float64, error) {
	db, prefix := GetDb()
	key := prefix + rk.StoragePrefix() + rk.StorageId()
	res := db.ZScore(ctx, key, member)
	if err := res.Err(); err != nil {
		return 0, err
	}
	return res.Val(), nil
}

// FetchListRange returns the subsegment of the "list of strings" value of RedisKey rk
// delimited by the given start and stop indices
func FetchListRange[K RedisKey](ctx context.Context, rk K, start int64, end int64) ([]string, error) {
	db, prefix := GetDb()
	key := prefix + rk.StoragePrefix() + rk.StorageId()
	res := db.LRange(ctx, key, start, end)
	if err := res.Err(); err != nil {
		return nil, err
	}
	return res.Val(), nil
}

// FetchListMemberBlocking fetches the leftmost or rightmost (per onLeft) element
// of the "list of strings" value of the RedisKey rk, blocking until there is an element
// to fetch or the timeout has expired.
//
// If the timeout expires, the empty string is returned.
//
// A fetched element is not removed from the list, but it is moved to the
// other end of the list, so if there are multiple elements in the list
// making the same call again will fetch a different element. The idea
// is that you process the element returned and then explicitly remove
// it from the list so it doesn't get fetched again.
func FetchListMemberBlocking[K RedisKey](ctx context.Context, rk K, onLeft bool, timeout time.Duration) (string, error) {
	db, prefix := GetDb()
	key := prefix + rk.StoragePrefix() + rk.StorageId()
	src, dst := "right", "left"
	if onLeft {
		src, dst = "left", "right"
	}
	res := db.BLMove(ctx, key, key, src, dst, timeout)
	if err := res.Err(); err != nil {
		return "", err
	}
	return res.Val(), nil
}

// MoveListMember moves the leftmost or rightmost (per srcLeft) element
// of the "list of strings" value of RedisKey src
// to be the leftmost or rightmost (per dstLeft) element
// of the "list of strings" value of the RedisKey dst.
func MoveListMember[K RedisKey](ctx context.Context, src K, dst K, srcLeft bool, dstLeft bool) (string, error) {
	db, prefix := GetDb()
	srcKey := prefix + src.StoragePrefix() + src.StorageId()
	dstKey := prefix + dst.StoragePrefix() + dst.StorageId()
	srcSide, dstSide := "right", "right"
	if srcLeft {
		srcSide = "left"
	}
	if dstLeft {
		dstSide = "left"
	}
	res := db.LMove(ctx, srcKey, dstKey, srcSide, dstSide)
	if err := res.Err(); err != nil {
		return "", err
	}
	return res.Val(), nil
}

// PushListMembers pushes the designated members onto the left or right end (per onLeft)
// of the "list of strings" value of RedisKey rk. The push is done "one at a time",
// so if you are pushing onto the left of the list the order of the members will
// be reversed relative to their order in the argument list.
func PushListMembers[K RedisKey](ctx context.Context, rk K, onLeft bool, members ...string) error {
	db, prefix := GetDb()
	key := prefix + rk.StoragePrefix() + rk.StorageId()
	args := make([]interface{}, len(members))
	for i, member := range members {
		args[i] = any(member)
	}
	var res *redis.IntCmd
	if onLeft {
		res = db.LPush(ctx, key, args...)
	} else {
		res = db.RPush(ctx, key, args...)
	}
	if err := res.Err(); err != nil {
		return err
	}
	return nil
}

// RemoveListElement removes up to count instances of the value element from the
// "list of strings" value of RedisKey rk. The search/delete happens left
// to right in the list. A count of 0 means "all of them".
func RemoveListElement[K RedisKey](ctx context.Context, rk K, count int64, element string) error {
	db, prefix := GetDb()
	key := prefix + rk.StoragePrefix() + rk.StorageId()
	res := db.LRem(ctx, key, count, any(element))
	if err := res.Err(); err != nil {
		return err
	}
	return nil
}

// GetMapValue returns the value of the key k from the "string->string map" value
// of RedisKey rk. If k is not in the map, then the empty string is returned.
func GetMapValue[K RedisKey](ctx context.Context, rk K, k string) (string, error) {
	db, prefix := GetDb()
	key := prefix + rk.StoragePrefix() + rk.StorageId()
	res := db.HGet(ctx, key, k)
	if err := res.Err(); err != nil {
		if errors.Is(err, redis.Nil) {
			return "", nil
		}
		return "", err
	}
	return res.Val(), nil
}

// SetMapValue maps k to v in the "string->string map" value
// of RedisKey rk. Any prior value of rk is replaced.
func SetMapValue[K RedisKey](ctx context.Context, rk K, k string, v string) error {
	db, prefix := GetDb()
	key := prefix + rk.StoragePrefix() + rk.StorageId()
	res := db.HSet(ctx, key, k, v)
	if err := res.Err(); err != nil {
		return err
	}
	return nil
}

// GetMapKeys returns all the keys from the "string->string map" value
// of RedisKey rk.
func GetMapKeys[K RedisKey](ctx context.Context, rk K) ([]string, error) {
	db, prefix := GetDb()
	key := prefix + rk.StoragePrefix() + rk.StorageId()
	res := db.HKeys(ctx, key)
	if err := res.Err(); err != nil {
		return nil, err
	}
	return res.Val(), nil
}

// GetMapAll returns the current "string->string map" value of RedisKey rk.
func GetMapAll[K RedisKey](ctx context.Context, rk K) (map[string]string, error) {
	db, prefix := GetDb()
	key := prefix + rk.StoragePrefix() + rk.StorageId()
	res := db.HGetAll(ctx, key)
	if err := res.Err(); err != nil {
		return nil, err
	}
	return res.Val(), nil
}

// MapRemoveKey deletes the key k from the "string->string map" value of the RedisKey rk.
//
// Removing a non-existent key is a no-op.
func MapRemoveKey[K RedisKey](ctx context.Context, rk K, k string) error {
	db, prefix := GetDb()
	key := prefix + rk.StoragePrefix() + rk.StorageId()
	res := db.HDel(ctx, key, k)
	if err := res.Err(); err != nil {
		return err
	}
	return nil
}

// A NotFoundError is returned as a "wrapped error"
// from all functions which retrieve a RedisValue
// from a RedisKey if the given key has no value.
// The error instance returned will have a message about which key failed,
// and it will satisfy errors.Is(err, NotFoundError).
var NotFoundError = errors.New("not found")

// FetchValueAtKey fills RedisValue v with the value stored at RedisKey rk in the database.
//
// See NotFoundError for what happens if k has no value.
func FetchValueAtKey[K RedisKey, V RedisValue](ctx context.Context, k K, v V) error {
	db, prefix := GetDb()
	key := prefix + k.StoragePrefix() + k.StorageId()
	bytes, err := db.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return fmt.Errorf("key %v: %w", key, NotFoundError)
	}
	if err != nil {
		return err
	}
	return v.FromRedis(bytes)
}

// StoreValueAtKey stores the RedisValue rv at the RedisKey rk in the database.
func StoreValueAtKey[K RedisKey, V RedisValue](ctx context.Context, rk K, rv V) error {
	db, prefix := GetDb()
	key := prefix + rk.StoragePrefix() + rk.StorageId()
	bytes, err := rv.ToRedis()
	if err != nil {
		return err
	}
	return db.Set(ctx, key, bytes, 0).Err()
}

// MapKeys runs f over every key in the database whose StoragePrefix matches that of
// RedisKey rk.
//
// If f returns an error, the iteration stops and that error is returned
// wrapped by an error which explains which key and StorageId was being processed.
func MapKeys[K RedisKey](ctx context.Context, f func(string) error, k K) error {
	db, prefix := GetDb()
	iter := db.Scan(ctx, 0, prefix+k.StoragePrefix()+"*", 20).Iterator()
	for iter.Next(ctx) {
		key := iter.Val()
		id := strings.TrimPrefix(key, prefix+k.StoragePrefix())
		if err := f(id); err != nil {
			return fmt.Errorf("process key %q, id %q: %w", key, id, err)
		}
	}
	return nil
}

// MapStringsAtKeys runs f over every key, value pair in the database whose StoragePrefix
// matches RedisKey rk. It is an error if value of each such key is not a string.
//
// If f returns an error, the iteration stops and that error is returned
// wrapped by an error which explains which key and StorageId was being processed.
func MapStringsAtKeys[K RedisKey](ctx context.Context, f func(string, string) error, k K) error {
	db, prefix := GetDb()
	iter := db.Scan(ctx, 0, prefix+k.StoragePrefix()+"*", 20).Iterator()
	for iter.Next(ctx) {
		key := iter.Val()
		id := strings.TrimPrefix(key, prefix+k.StoragePrefix())
		val, err := db.Get(ctx, key).Result()
		if err != nil {
			return fmt.Errorf("fetch key %q: %w", key, err)
		}
		if err = f(id, val); err != nil {
			return fmt.Errorf("process key %q, id %q, val %q: %w", key, id, val, err)
		}
	}
	return nil
}

// MapValuesAtKeys finds every key in the database whose StoragePrefix matches that of
// RedisKey rk, fills RedisValue v with the value found for that key, and then invokes
// f (which is expected to be a closure over v). It is an error if one of the values
// found is not of v's concrete type.
//
// If f returns an error, the iteration stops and that error is returned
// wrapped by an error which explains which key and StorageId was being processed.
func MapValuesAtKeys[K RedisKey, V RedisValue](ctx context.Context, f func() error, k K, v V) error {
	db, prefix := GetDb()
	iter := db.Scan(ctx, 0, prefix+k.StoragePrefix()+"*", 20).Iterator()
	for iter.Next(ctx) {
		key := iter.Val()
		bytes, err := db.Get(ctx, key).Bytes()
		if err != nil {
			return fmt.Errorf("fetch key %s: %w", key, err)
		}
		err = v.FromRedis(bytes)
		if err != nil {
			return fmt.Errorf("unmarshal key %s: %w", key, err)
		}
		if err = f(); err != nil {
			return fmt.Errorf("process key %s, value %v: %w", key, v, err)
		}
	}
	return nil
}

// An Object is both a RedisKey and a RedisValue.
type Object interface {
	RedisKey
	RedisValue
}

// FetchObject loads the value of RedisKey obj into RedisValue obj.
func FetchObject[T Object](ctx context.Context, obj T) error {
	return FetchValueAtKey(ctx, obj, obj)
}

// StoreObject saves the value of RedisValue obj into the database at RedisKey obj.
func StoreObject[T Object](ctx context.Context, obj T) error {
	return StoreValueAtKey(ctx, obj, obj)
}

// MapObjects treats obj as both a RedisKey and a RedisValue and calls MapValuesAtKeys with f.
func MapObjects[T Object](ctx context.Context, f func() error, obj T) error {
	return MapValuesAtKeys(ctx, f, obj, obj)
}

// StorableString turns a string into a RedisKey with a string value.
type StorableString string

func (s StorableString) StoragePrefix() string {
	return "string:"
}

func (s StorableString) StorageId() string {
	return string(s)
}

// StorableSet turns a string into a RedisKey with "set of strings" value.
type StorableSet string

func (s StorableSet) StoragePrefix() string {
	return "set:"
}

func (s StorableSet) StorageId() string {
	return string(s)
}

// StorableSortedSet turns a string into a RedisKey with a "sorted set of strings" value
type StorableSortedSet string

func (s StorableSortedSet) StoragePrefix() string {
	return "zset:"
}

func (s StorableSortedSet) StorageId() string {
	return string(s)
}

// StorableList turns a string into a RedisKey with a "list of strings" value.
type StorableList string

func (s StorableList) StoragePrefix() string {
	return "list:"
}

func (s StorableList) StorageId() string {
	return string(s)
}

// StorableMap turns a string into a RedisKey with "string->string map" value.
type StorableMap string

func (s StorableMap) StoragePrefix() string {
	return "map:"
}

func (s StorableMap) StorageId() string {
	return string(s)
}
