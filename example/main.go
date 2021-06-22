package main

import (
	"context"
	"fmt"
	"github.com/hunterhug/gorlock"
	"time"
)

func main() {
	gorlock.SetDebug()

	// 1. config redis
	// 1. 配置Redis
	redisHost := "127.0.0.1:6379"
	redisDb := 0
	redisPass := "root" // may redis has password
	config := gorlock.NewRedisSingleModeConfig(redisHost, redisDb, redisPass)
	pool, err := gorlock.NewRedisPool(config)
	if err != nil {
		fmt.Println("redis init err:", err.Error())
		return
	}

	// 2. new a lock factory
	// 2. 新建一个锁工厂
	lockFactory := gorlock.New(pool)

	// set retry time and delay mill second
	// 设置锁重试次数和尝试等待时间，表示加锁不成功等200毫秒再尝试，总共尝试3次，设置负数表示一直尝试，会堵住
	lockFactory.SetRetryCount(3).SetRetryMillSecondDelay(200)

	// keep alive will auto extend the expire time of lock
	// 自动续命锁
	lockFactory.SetKeepAlive(true)

	// 3. lock a resource
	// 3. 锁住资源，资源名为myLock
	resourceName := "myLock"

	// 10s
	// 过期时间10秒
	expireMillSecond := 10 * 1000
	lock, err := lockFactory.Lock(context.Background(), resourceName, expireMillSecond)
	if err != nil {
		fmt.Println("lock err:", err.Error())
		return
	}

	// lock empty point not lock
	// 锁为空，表示加锁失败
	if lock == nil {
		fmt.Println("lock err:", "can not lock")
		return
	}

	// add lock success
	// 加锁成功
	fmt.Printf("add lock success:%#v\n", lock)

	time.Sleep(3 * time.Second)

	// 4. unlock a resource
	// 4. 解锁
	isUnlock, err := lockFactory.UnLock(context.Background(), lock)
	if err != nil {
		fmt.Println("lock err:", err.Error())
		return
	}

	fmt.Printf("lock unlock: %v, %#v\n", isUnlock, lock)

	// unlock a resource again no effect side
	// 上面已经解锁了，可以多次解锁
	isUnlock, err = lockFactory.UnLock(context.Background(), lock)
	if err != nil {
		fmt.Println("lock err:", err.Error())
		return
	}

	fmt.Println("lock unlock:", isUnlock)
}
