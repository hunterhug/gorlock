package main

import (
	"fmt"
	"github.com/hunterhug/rlock"
)

func main() {
	// 1. config redis
	redisHost := "127.0.0.1:6379"
	redisDb := 0
	redisPass := "hunterhug" // may redis has password
	config := rlock.NewRedisSingleModeConfig(redisHost, redisDb, redisPass)
	pool, err := rlock.NewRedisPool(config)
	if err != nil {
		fmt.Println("redis init err:", err.Error())
		return
	}

	// 2. new a lock factory
	lockFactory := rlock.New(pool)
	lockFactory.SetRetryCount(3).SetRetryMillSecondDelay(200) // set retry time and delay mill second

	// 3. lock a resource
	resourceName := "gold"
	expireMillSecond := 10 * 1000 // 10s
	lock, err := lockFactory.Lock(resourceName, expireMillSecond)
	if err != nil {
		fmt.Println("lock err:", err.Error())
		return
	}

	// lock empty point not lock
	if lock == nil {
		fmt.Println("lock err:", "can not lock")
		return
	}

	// add lock success
	fmt.Printf("add lock success:%#v\n", lock)

	// 4. unlock a resource
	isUnlock, err := lockFactory.UnLock(*lock)
	if err != nil {
		fmt.Println("lock err:", err.Error())
		return
	}

	fmt.Println("lock unlock:", isUnlock)

	// unlock a resource again
	isUnlock, err = lockFactory.UnLock(*lock)
	if err != nil {
		fmt.Println("lock err:", err.Error())
		return
	}

	fmt.Println("lock unlock:", isUnlock)
}
