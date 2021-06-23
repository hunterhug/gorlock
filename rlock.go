/*
	All right reserved：https://github.com/hunterhug/gorlock at 2020
	Attribution-NonCommercial-NoDerivatives 4.0 International
	You can use it for education only but can't make profits for any companies and individuals!
*/
package gorlock // import "github.com/hunterhug/gorlock"

import (
	"context"
	"errors"
	uuid "github.com/gofrs/uuid"
	"github.com/gomodule/redigo/redis"
	"github.com/hunterhug/gosession/kv"
	"strings"
	"sync"
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
	DefaultRetryCount                 = 3
	DefaultUnlockRetryCount           = 3
	DefaultRetryMillSecondDelay       = 200
	DefaultUnlockRetryMillSecondDelay = 200
)

var (
	LockEmptyError  = errors.New("lock empty")
	luaDelScript    = redis.NewScript(1, `if redis.call('get', KEYS[1]) == ARGV[1] then return redis.call('del', KEYS[1]) else return 0 end`)
	luaExpireScript = redis.NewScript(2, `if redis.call('get', KEYS[1]) == ARGV[1] then return redis.call('PEXPIRE', KEYS[2], ARGV[2]) else return 0 end`)
)

// Redis lock factory
type RedisLockFactory struct {
	pool                       *redis.Pool // redis pool can single mode or other mode
	retryLockTryCount          int64
	retryLockMillSecondDelay   int64
	retryUnLockTryCount        int64
	retryUnLockMillSecondDelay int64
	isKeepAlive                bool
}

// Lock
type Lock struct {
	CreateMillSecondTime        int64
	LiveMillSecondTime          int
	LastKeepAliveMillSecondTime int64
	UnlockMillSecondTime        int64
	ResourceName                string
	RandomKey                   string
	IsUnlock                    bool
	closeChan                   chan struct{}
	closeChanHasBeenFull        bool
	isKeepAlive                 bool
	mutex                       sync.Mutex
}

func New(pool *redis.Pool) *RedisLockFactory {
	return &RedisLockFactory{
		pool:                       pool,
		retryLockTryCount:          DefaultRetryCount,
		retryLockMillSecondDelay:   DefaultRetryMillSecondDelay,
		retryUnLockTryCount:        DefaultUnlockRetryCount,
		retryUnLockMillSecondDelay: DefaultUnlockRetryMillSecondDelay,
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
	s.retryLockTryCount = c
	return s
}

func (s *RedisLockFactory) SetUnLockRetryCount(c int64) *RedisLockFactory {
	if c < 0 {
		c = DefaultUnlockRetryCount
	}
	s.retryUnLockTryCount = c
	return s
}

func (s *RedisLockFactory) SetKeepAlive(isKeepAlive bool) *RedisLockFactory {
	s.isKeepAlive = isKeepAlive
	return s
}

func (s *RedisLockFactory) SetRetryMillSecondDelay(c int64) *RedisLockFactory {
	if c < 0 {
		c = DefaultRetryMillSecondDelay
	}
	s.retryLockMillSecondDelay = c
	return s
}

func (s *RedisLockFactory) SetUnLockRetryMillSecondDelay(c int64) *RedisLockFactory {
	if c < 0 {
		c = DefaultRetryMillSecondDelay
	}
	s.retryUnLockMillSecondDelay = c
	return s
}

func (s *RedisLockFactory) Lock(ctx context.Context, resourceName string, lockMillSecond int) (lock *Lock, err error) {
	conn := s.pool.Get()
	defer conn.Close()
	lockTime := lockMillSecond + 1

	randomKey := GetGUID()
	retry := s.retryLockTryCount + 1

	for retry > 0 || retry <= 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
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
						time.Sleep(time.Duration(s.retryLockMillSecondDelay) * time.Millisecond)
						continue
					}
					return nil, err
				}
			}

			if rep == "OK" {
				success = true
			}

			if !success {
				Logger.Debugf("start in %v lock %s with random key:%s, expire: %d ms not lock", beginTime, resourceName, randomKey, lockMillSecond)
				if retry != 0 {
					time.Sleep(time.Duration(s.retryLockMillSecondDelay) * time.Millisecond)
					continue
				}
				return nil, nil
			}

			lock = new(Lock)
			lock.LiveMillSecondTime = lockTime
			lock.RandomKey = randomKey
			lock.ResourceName = resourceName
			lock.CreateMillSecondTime = time.Now().UnixNano() / 1e6
			if s.isKeepAlive {
				lock.isKeepAlive = true
				lock.closeChan = make(chan struct{})
				go s.keepAlive(lock)
			}
			return lock, nil
		}
	}

	return nil, LockEmptyError
}

func (s *RedisLockFactory) UnLock(ctx context.Context, lock *Lock) (isUnLock bool, err error) {
	if lock == nil {
		return false, LockEmptyError
	}

	lock.mutex.Lock()
	defer lock.mutex.Unlock()

	if lock.IsUnlock {
		return true, nil
	}

	if lock.isKeepAlive {
		if !lock.closeChanHasBeenFull {
			lock.closeChan <- struct{}{}
			lock.closeChanHasBeenFull = true
		}
	} else {
		lock.UnlockMillSecondTime = time.Now().UnixNano() / 1e6
		lock.IsUnlock = true
	}

	conn := s.pool.Get()
	defer conn.Close()

	retry := s.retryUnLockTryCount + 1

	for retry > 0 {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		default:
			beginTime := time.Now().UnixNano()
			retry -= 1
			rep, err := redis.Int64(luaDelScript.Do(conn, lock.ResourceName, lock.RandomKey))
			if err != nil {
				Logger.Debugf("start in %v unlock %s with random key:%s, err:%s", beginTime, lock.ResourceName, lock.RandomKey, err.Error())
				if retry != 0 {
					time.Sleep(time.Duration(s.retryUnLockMillSecondDelay) * time.Millisecond)
					continue
				}
				return false, err
			}

			if rep != 1 {
				Logger.Debugf("start in %v unlock %s with random key:%s not unlock", beginTime, lock.ResourceName, lock.RandomKey)
				return false, nil
			}

			return true, nil
		}
	}

	return false, LockEmptyError
}

func (s *RedisLockFactory) keepAlive(lock *Lock) {
	conn := s.pool.Get()
	defer conn.Close()

	for {
		if lock.IsUnlock {
			return
		}

		select {
		case <-lock.closeChan:
			beginTime := time.Now().UnixNano()
			Logger.Debugf("start in %v keepAlive %s with random key:%s close", beginTime, lock.ResourceName, lock.RandomKey)
			lock.IsUnlock = true
			lock.UnlockMillSecondTime = time.Now().UnixNano() / 1e6
			close(lock.closeChan)
			return
		case <-time.After(500 * time.Millisecond):
			beginTime := time.Now().UnixNano()

			i, err := luaExpireScript.Do(conn, lock.ResourceName, lock.ResourceName, lock.RandomKey, lock.LiveMillSecondTime)
			rep, err := redis.Int64(i, err)
			if err != nil {
				Logger.Debugf("start in %v keepAlive %s with random key:%s, err:%s", beginTime, lock.ResourceName, lock.RandomKey, err.Error())
				continue
			}

			if rep != 1 {
				Logger.Debugf("start in %v keepAlive %s with random key:%s not keepAlive=%v", beginTime, lock.ResourceName, lock.RandomKey, rep)
				lock.IsUnlock = true
				lock.UnlockMillSecondTime = time.Now().UnixNano() / 1e6
				close(lock.closeChan)
				return
			}

			lock.LastKeepAliveMillSecondTime = time.Now().UnixNano() / 1e6
			Logger.Debugf("start in %v keepAlive %s with random key:%s doing", beginTime, lock.ResourceName, lock.RandomKey)
		}
	}
}
