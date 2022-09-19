package gorlock // import "github.com/hunterhug/gorlock"

import (
	"context"
	"errors"
	"github.com/gomodule/redigo/redis"
	"github.com/hunterhug/gosession"
	"github.com/hunterhug/gosession/kv"
	"io"
	"strings"
	"sync"
	"time"
)

const (
	defaultRetryCount                 = 3
	defaultUnlockRetryCount           = 3
	defaultRetryMillSecondDelay       = 200
	defaultUnlockRetryMillSecondDelay = 200
)

var (
	LockEmptyError  = errors.New("lock empty")
	luaDelScript    = redis.NewScript(1, `if redis.call('get', KEYS[1]) == ARGV[1] then return redis.call('del', KEYS[1]) else return 0 end`)
	luaExpireScript = redis.NewScript(2, `if redis.call('get', KEYS[1]) == ARGV[1] then return redis.call('PEXPIRE', KEYS[2], ARGV[2]) else return 0 end`)
)

// LockFactory is main interface
type LockFactory interface {
	SetRetryCount(c int64) LockFactory
	GetRetryCount() int64
	SetUnLockRetryCount(c int64) LockFactory
	GetUnLockRetryCount() int64
	SetKeepAlive(isKeepAlive bool) LockFactory
	IsKeepAlive() bool
	SetRetryMillSecondDelay(c int64) LockFactory
	GetRetryMillSecondDelay() int64
	SetUnLockRetryMillSecondDelay(c int64) LockFactory
	GetUnLockRetryMillSecondDelay() int64
	LockFactoryCore
}

// LockFactoryCore is core interface
type LockFactoryCore interface {
	// Lock ,lock resource default keepAlive depend on LockFactory
	Lock(ctx context.Context, resourceName string, lockMillSecond int) (lock *Lock, err error)
	// LockForceKeepAlive lock resource force keepAlive
	LockForceKeepAlive(ctx context.Context, resourceName string, lockMillSecond int) (lock *Lock, err error)
	// LockForceNotKeepAlive lock resource force not keepAlive
	LockForceNotKeepAlive(ctx context.Context, resourceName string, lockMillSecond int) (lock *Lock, err error)
	// UnLock ,unlock resource
	UnLock(ctx context.Context, lock *Lock) (isUnLock bool, err error)
	// Done asynchronous see lock whether is release
	Done(lock *Lock) chan struct{}
}

// redisLockFactory Redis lock factory
type redisLockFactory struct {
	// redis pool can single mode or other mode
	pool *redis.Pool

	// as the name say
	retryLockTryCount          int64
	retryLockMillSecondDelay   int64
	retryUnLockTryCount        int64
	retryUnLockMillSecondDelay int64

	// auto keep alive the lock util call unlock
	isKeepAlive bool
}

// Lock Redis lock
type Lock struct {
	// lock time
	CreateMillSecondTime int64

	// lock live time, TTL
	LiveMillSecondTime int

	// if isKeepAlive, the last time refresh lock live time
	LastKeepAliveMillSecondTime int64

	// unlock time
	UnlockMillSecondTime int64

	// resource you lock
	ResourceName string

	// unique key of resource lock
	RandomKey string

	// when lock is unlock, will make chan signal, keepAlive find it unlock or call UnLock()
	IsUnlock     bool
	isUnlockChan chan struct{}

	// call UnLock() will set to true
	IsClose bool

	// the lock is open keep alive
	OpenKeepAlive bool

	// which Unlock() send to keepAlive()
	closeChan chan struct{}

	// which keepAlive() response to Unlock()
	keepAliveDealCloseChan chan bool

	// lock something
	mutex sync.Mutex
}

// Done judge lock is whether release
// one example is:
//
//	lockFactory.SetKeepAlive(true).SetRetryCount(-1)
//	lock, _ := lockFactory.Lock(context.Background(), resourceName, expireMillSecond)
//	if lock != nil {
//		crontab = true
//	}
//	select {
//		case <- lock.Done():
//			crontab = false
//	}
//
// very useful for crontab work
func (lock *Lock) Done() chan struct{} {
	return lock.isUnlockChan
}

// New make a redis lock factory
// call NewRedisSingleModeConfig and NewRedisSentinelModeConfig then call NewRedisPool
// you can also call kv.NewRedis for diy redis pool
func New(pool *redis.Pool) LockFactory {
	if pool.TestOnBorrow != nil {
		pool.TestOnBorrow = func(c redis.Conn, t time.Time) error {
			if time.Since(t) < time.Minute {
				return nil
			}

			_, err := c.Do("PING")
			if err != nil {
				Logger.Debugf("redis ping err: %s", err.Error())
			}
			return err
		}
	}

	factory := &redisLockFactory{
		pool:                       pool,
		retryLockTryCount:          defaultRetryCount,
		retryLockMillSecondDelay:   defaultRetryMillSecondDelay,
		retryUnLockTryCount:        defaultUnlockRetryCount,
		retryUnLockMillSecondDelay: defaultUnlockRetryMillSecondDelay,
	}

	return factory
}

// NewByRedisConfig One step do everyThing
func NewByRedisConfig(redisConf *kv.MyRedisConf) (LockFactory, error) {
	pool, err := NewRedisPool(redisConf)
	if err != nil {
		return nil, err
	}

	factory := New(pool)
	return factory, nil
}

// NewRedisPool make a redis pool
var NewRedisPool = kv.NewRedis

// NewRedisSingleModeConfig redis single mode config
func NewRedisSingleModeConfig(redisHost string, redisDB int, redisPass string) *kv.MyRedisConf {
	return &kv.MyRedisConf{
		RedisHost:          redisHost,
		RedisPass:          redisPass,
		RedisDB:            redisDB,
		RedisIdleTimeout:   360,
		RedisMaxActive:     0,
		RedisMaxIdle:       0,
		DialConnectTimeout: 20,
		DialReadTimeout:    20,
		DialWriteTimeout:   20,
	}
}

// NewRedisSentinelModeConfig redis sentinel mode config
// redisHost is sentinel address, not redis address
// redisPass is redis password
func NewRedisSentinelModeConfig(redisHost string, redisDB int, redisPass string, masterName string) *kv.MyRedisConf {
	return &kv.MyRedisConf{
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

// Done judge lock is whether release
func (s *redisLockFactory) Done(lock *Lock) chan struct{} {
	return lock.Done()
}

// SetRetryCount if lock fail, can try again
// if c=0 point not retry, c=1 try ones, and so on.
// if c<0 there will try loop to world die.
func (s *redisLockFactory) SetRetryCount(c int64) LockFactory {
	s.retryLockTryCount = c
	return s
}

func (s *redisLockFactory) GetRetryCount() int64 {
	return s.retryLockTryCount
}

// SetUnLockRetryCount as the SetRetryCount, but is reverse.
func (s *redisLockFactory) SetUnLockRetryCount(c int64) LockFactory {
	if c < 0 {
		c = defaultUnlockRetryCount
	}
	s.retryUnLockTryCount = c
	return s
}

func (s *redisLockFactory) GetUnLockRetryCount() int64 {
	return s.retryUnLockTryCount
}

// SetKeepAlive keep alive the lock
// useful when your work waste a lot of time, you can keep lock alive prevent others grab your lock
func (s *redisLockFactory) SetKeepAlive(isKeepAlive bool) LockFactory {
	s.isKeepAlive = isKeepAlive
	return s
}

func (s *redisLockFactory) IsKeepAlive() bool {
	return s.isKeepAlive
}

// SetRetryMillSecondDelay retry must delay some times
func (s *redisLockFactory) SetRetryMillSecondDelay(c int64) LockFactory {
	if c < 0 {
		c = defaultRetryMillSecondDelay
	}
	s.retryLockMillSecondDelay = c
	return s
}

func (s *redisLockFactory) GetRetryMillSecondDelay() int64 {
	return s.retryLockMillSecondDelay
}

// SetUnLockRetryMillSecondDelay retry must delay sometimes
func (s *redisLockFactory) SetUnLockRetryMillSecondDelay(c int64) LockFactory {
	if c < 0 {
		c = defaultRetryMillSecondDelay
	}
	s.retryUnLockMillSecondDelay = c
	return s
}

func (s *redisLockFactory) GetUnLockRetryMillSecondDelay() int64 {
	return s.retryUnLockMillSecondDelay
}

// Lock ,lock the resource with lockMillSecond TTL(Time to Live) time
// take context.Context to control time out lock when try many times
func (s *redisLockFactory) Lock(ctx context.Context, resourceName string, lockMillSecond int) (lock *Lock, err error) {
	return s.lock(ctx, resourceName, lockMillSecond, true, false)
}

// LockForceKeepAlive lock force keepAlive, must call defer Unlock()
func (s *redisLockFactory) LockForceKeepAlive(ctx context.Context, resourceName string, lockMillSecond int) (lock *Lock, err error) {
	return s.lock(ctx, resourceName, lockMillSecond, false, true)
}

// LockForceNotKeepAlive lock force not keepalive
func (s *redisLockFactory) LockForceNotKeepAlive(ctx context.Context, resourceName string, lockMillSecond int) (lock *Lock, err error) {
	return s.lock(ctx, resourceName, lockMillSecond, false, false)
}

func IsConnError(err error) bool {
	var needNewConn bool

	if err == nil {
		return false
	}

	if err == io.EOF {
		needNewConn = true
	}
	if strings.Contains(err.Error(), "use of closed network connection") {
		needNewConn = true
	}
	if strings.Contains(err.Error(), "connect: connection refused") {
		needNewConn = true
	}
	return needNewConn
}

func (s *redisLockFactory) lock(ctx context.Context, resourceName string, lockMillSecond int, useFactoryKeepAlive bool, keepAlive bool) (lock *Lock, err error) {
	// get redis con
	conn := s.pool.Get()

	defer func(conn redis.Conn) {
		err := conn.Close()
		if err != nil {
			// ignore
		}
	}(conn)

	// ttl add one second for magic
	lockTime := lockMillSecond + 1

	randomKey := gosession.GetGUID()
	retry := s.retryLockTryCount + 1

	for retry > 0 || retry <= 0 {
		select {
		// if time out, return
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			beginTime := time.Now().String()
			retry -= 1

			rep, err := redis.String(conn.Do("set", resourceName, randomKey, "nx", "px", lockTime))
			success := false

			if err != nil {
				if IsConnError(err) {
					Logger.Debugf("lock start in %v lock %s with random key:%s, expire: %d ms err:%s", beginTime, resourceName, randomKey, lockMillSecond, err.Error())
					return nil, err
				}

				// after Redis 2.6.12, set nx will return nil reply if condition not match
				if err == redis.ErrNil {
				} else {
					// redis inner err

					// debug you can SetLogLevel("DEBUG")
					Logger.Debugf("lock start in %v lock %s with random key:%s, expire: %d ms err:%s", beginTime, resourceName, randomKey, lockMillSecond, err.Error())

					// retry again
					if retry != 0 {
						time.Sleep(time.Duration(s.retryLockMillSecondDelay) * time.Millisecond)
						continue
					}
					return nil, err
				}
			}

			if rep == "OK" {
				// success we go
				success = true
			}

			if !success {
				// lock is hold by others
				Logger.Debugf("lock start in %v lock %s with random key:%s, expire: %d ms not lock", beginTime, resourceName, randomKey, lockMillSecond)
				if retry != 0 {
					// try again
					time.Sleep(time.Duration(s.retryLockMillSecondDelay) * time.Millisecond)
					continue
				}
				return nil, nil
			}

			// lock is success gen, make a Lock return
			lock = new(Lock)
			lock.LiveMillSecondTime = lockTime
			lock.RandomKey = randomKey
			lock.ResourceName = resourceName
			lock.CreateMillSecondTime = time.Now().UnixNano() / 1e6

			// when lock unlock, send a chan to caller
			lock.isUnlockChan = make(chan struct{}, 1)

			if useFactoryKeepAlive {
				// if is keepAlive, we make a new goroutine to do the work
				if s.isKeepAlive {
					lock.OpenKeepAlive = true
					lock.closeChan = make(chan struct{})
					lock.keepAliveDealCloseChan = make(chan bool)
					go s.keepAlive(lock)
				}
			} else {
				if keepAlive {
					lock.OpenKeepAlive = true
					lock.closeChan = make(chan struct{})
					lock.keepAliveDealCloseChan = make(chan bool)
					go s.keepAlive(lock)
				}
			}

			return lock, nil
		}
	}

	return nil, LockEmptyError
}

// UnLock ,unlock the resource, but now we should pass variable "lock Lock" into func but not resource name
// suggest call defer UnLock() after Lock()
func (s *redisLockFactory) UnLock(ctx context.Context, lock *Lock) (isUnLock bool, err error) {
	if lock == nil {
		return false, LockEmptyError
	}

	// avoid unlock the same, mutex it
	lock.mutex.Lock()
	defer lock.mutex.Unlock()

	// only can call unlock once
	if lock.IsClose {
		return true, nil
	}

	lock.IsClose = true

	if lock.OpenKeepAlive {
		Logger.Debug("UnLock send signal to keepAlive ask game over")

		// will block util keepAlive deal
		lock.closeChan <- struct{}{}

		keepAliveHasBeenClose := <-lock.keepAliveDealCloseChan

		if keepAliveHasBeenClose {
			Logger.Debugf("UnLock receive signal to keepAlive response keepAliveHasBeenClose=%v UnLock direct return", keepAliveHasBeenClose)
			return true, nil
		}

		Logger.Debugf("UnLock receive signal to keepAlive response keepAliveHasBeenClose=%v", keepAliveHasBeenClose)

	} else {
		lock.IsUnlock = true
		lock.UnlockMillSecondTime = time.Now().UnixNano() / 1e6

		// close a close chan
		close(lock.closeChan)

		// unlock chan send
		lock.isUnlockChan <- struct{}{}
	}

	conn := s.pool.Get()
	defer func(conn redis.Conn) {
		err := conn.Close()
		if err != nil {
			// ignore
		}
	}(conn)

	retry := s.retryUnLockTryCount + 1

	for retry > 0 {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		default:
			beginTime := time.Now().String()
			retry -= 1
			rep, err := redis.Int64(luaDelScript.Do(conn, lock.ResourceName, lock.RandomKey))
			if err != nil {
				Logger.Debugf("UnLock start in %v unlock %s with random key:%s, err:%s", beginTime, lock.ResourceName, lock.RandomKey, err.Error())

				if IsConnError(err) {
					return false, err
				}

				if retry != 0 {
					time.Sleep(time.Duration(s.retryUnLockMillSecondDelay) * time.Millisecond)
					continue
				}
				return false, err
			}

			if rep != 1 {
				// no err but the lock is not exist, may be expired or grab by others
				Logger.Debugf("UnLock start in %v unlock %s with random key:%s not unlock", beginTime, lock.ResourceName, lock.RandomKey)
				return false, nil
			}

			// unlock success
			return true, nil
		}
	}

	return false, LockEmptyError
}

func (s *redisLockFactory) keepAliveSendToRedis(lock *Lock) (int64, error) {
	conn := s.pool.Get()
	defer func(conn redis.Conn) {
		err := conn.Close()
		if err != nil {
			// ignore
		}
	}(conn)
	rep, err := redis.Int64(luaExpireScript.Do(conn, lock.ResourceName, lock.ResourceName, lock.RandomKey, lock.LiveMillSecondTime))
	return rep, err
}

func (s *redisLockFactory) keepAlive(lock *Lock) {
	isKeepAliveFail := false

	for {
		if isKeepAliveFail && !lock.IsClose {
			go func() {
				_, err := s.UnLock(context.Background(), lock)
				if err != nil {
					// ignore
				}
			}()
		}

		select {
		case <-lock.closeChan:
			// receive a close chan
			beginTime := time.Now().String()
			Logger.Debugf("start in %v keepAlive %s with random key:%s close", beginTime, lock.ResourceName, lock.RandomKey)
			if !isKeepAliveFail {
				lock.IsUnlock = true
				lock.UnlockMillSecondTime = time.Now().UnixNano() / 1e6
				lock.isUnlockChan <- struct{}{}
			}
			close(lock.closeChan)
			lock.keepAliveDealCloseChan <- isKeepAliveFail
			return
		case <-time.After(500 * time.Millisecond):
			if isKeepAliveFail {
				continue
			}

			beginTime := time.Now().String()

			rep, err := s.keepAliveSendToRedis(lock)
			if err != nil {
				// if redis inner err, retry forever
				Logger.Debugf("start in %v keepAlive %s with random key:%s, err:%s", beginTime, lock.ResourceName, lock.RandomKey, err.Error())
				continue
			}

			if rep != 1 {
				// lock not exist or others grab it
				Logger.Debugf("start in %v keepAlive %s with random key:%s not keepAlive=%v", beginTime, lock.ResourceName, lock.RandomKey, rep)

				lock.IsUnlock = true
				lock.UnlockMillSecondTime = time.Now().UnixNano() / 1e6
				lock.isUnlockChan <- struct{}{}
				isKeepAliveFail = true

				// will loop util call Unlock()
				continue
			}

			lock.LastKeepAliveMillSecondTime = time.Now().UnixNano() / 1e6
			Logger.Debugf("start in %v keepAlive %s with random key:%s doing", beginTime, lock.ResourceName, lock.RandomKey)
		}
	}
}

// SetDebug for debug
func SetDebug() {
	SetLogLevel(DEBUG)
}
