/*
	All right reserved：https://github.com/hunterhug/rlock at 2020
	Attribution-NonCommercial-NoDerivatives 4.0 International
	You can use it for education only but can't make profits for any companies and individuals!
*/
package rlock // import "github.com/hunterhug/rlock"
import (
	"errors"
	uuid "github.com/gofrs/uuid"
	"github.com/gomodule/redigo/redis"
	"github.com/hunterhug/gosession/kv"
	"strings"
	"time"
)

// gen random uuid
func GetGUID() (valueGUID string) {
	objID, _ := uuid.NewV4()
	objIdStr := objID.String()
	objIdStr = strings.Replace(objIdStr, "-", "", -1)
	valueGUID = objIdStr
	return valueGUID
}

// for debug
func SetDebug() {
	SetLogLevel("DEBUG")
}

const (
	DefaultRetryCount           = 3
	DefaultRetryMillSecondDelay = 200
)

var (
	LockEmptyError = errors.New("lock empty")
	luaDelScript   = redis.NewScript(1, `if redis.call('get', KEYS[1]) == ARGV[1] then return redis.call('del', KEYS[1]) else return 0 end`)
)

// Redis lock factory
type RedisLockFactory struct {
	pool                 *redis.Pool // redis pool can single mode or other mode
	retryCount           int64
	retryMillSecondDelay int64
}

// Lock
type Lock struct {
	LiveMillSecondTime int
	ResourceName       string
	RandomKey          string
}

func New(pool *redis.Pool) *RedisLockFactory {
	return &RedisLockFactory{
		pool:                 pool,
		retryCount:           DefaultRetryCount,
		retryMillSecondDelay: DefaultRetryMillSecondDelay,
	}
}

// redis single mode config
func NewRedisSingleModeConfig(redisHost string, redisDB int, redisPass string) kv.MyRedisConf {
	return kv.MyRedisConf{
		RedisHost:        redisHost,
		RedisPass:        redisPass,
		RedisDB:          redisDB,
		RedisIdleTimeout: 15,
		RedisMaxActive:   0,
		RedisMaxIdle:     0,
	}
}

// redis sentinel mode config
// redisHost is sentinel address, not redis address
// redisPass is redis password
func NewRedisSSentinelModeConfig(redisHost string, redisDB int, redisPass string, masterName string) kv.MyRedisConf {
	return kv.MyRedisConf{
		RedisHost:        redisHost,
		RedisDB:          redisDB,
		RedisIdleTimeout: 15,
		RedisMaxActive:   0,
		RedisMaxIdle:     0,
		IsCluster:        true,
		MasterName:       masterName,
		RedisPass:        redisPass,
	}
}

func NewRedisPool(redisConf kv.MyRedisConf) (pool *redis.Pool, err error) {
	return kv.NewRedis(&redisConf)
}

func (s *RedisLockFactory) SetRetryCount(c int64) *RedisLockFactory {
	if c < 0 {
		c = 0
	}
	s.retryCount = c
	return s
}

func (s *RedisLockFactory) SetRetryMillSecondDelay(c int64) *RedisLockFactory {
	if c < 0 {
		c = DefaultRetryMillSecondDelay
	}
	s.retryMillSecondDelay = c
	return s
}

func (s *RedisLockFactory) Lock(resourceName string, lockMillSecond int) (lock *Lock, err error) {
	conn := s.pool.Get()
	defer conn.Close()
	lockTime := lockMillSecond + 1

	randomKey := GetGUID()
	retry := s.retryCount + 1

	for retry > 0 {
		beginTime := time.Now().UnixNano()
		retry -= 1

		// SET resource_name my_random_value NX PX 30000
		rep, err := redis.String(conn.Do("set", resourceName, randomKey, "nx", "px", lockTime))
		success := false
		if err != nil {
			if err == redis.ErrNil {
			} else {
				Logger.Debugf("start in %v lock %s with random key:%s, expire: %d ms err:%s", beginTime, resourceName, randomKey, lockMillSecond, err.Error())
				if retry != 0 {
					time.Sleep(time.Duration(s.retryMillSecondDelay) * time.Millisecond)
					continue
				}
				return nil, err
			}
		} else {
			if rep == "OK" {
				success = true
			}
		}

		if !success {
			Logger.Debugf("start in %v lock %s with random key:%s, expire: %d ms not lock", beginTime, resourceName, randomKey, lockMillSecond)
			if retry != 0 {
				time.Sleep(time.Duration(s.retryMillSecondDelay) * time.Millisecond)
				continue
			}
			return nil, nil
		}

		lock = new(Lock)
		lock.LiveMillSecondTime = lockTime
		lock.RandomKey = randomKey
		lock.ResourceName = resourceName
		return lock, nil
	}

	return nil, LockEmptyError
}

func (s *RedisLockFactory) UnLock(lock Lock) (isUnLock bool, err error) {
	conn := s.pool.Get()
	defer conn.Close()

	retry := s.retryCount + 1

	for retry > 0 {
		beginTime := time.Now().UnixNano()
		retry -= 1
		rep, err := redis.Int64(luaDelScript.Do(conn, lock.ResourceName, lock.RandomKey))
		if err != nil {
			if retry != 0 {
				time.Sleep(time.Duration(s.retryMillSecondDelay) * time.Millisecond)
				continue
			}
			Logger.Debugf("start in %v unlock %s with random key:%s, err:%s", beginTime, lock.ResourceName, lock.RandomKey, err.Error())
			return false, err
		}

		if rep != 1 {
			Logger.Debugf("start in %v unlock %s with random key:%s not unlock", beginTime, lock.ResourceName, lock.RandomKey)
			return false, nil
		}

		return true, nil
	}

	return false, LockEmptyError
}