# Redis Distributed Lock By Golang

[![GitHub forks](https://img.shields.io/github/forks/hunterhug/gorlock.svg?style=social&label=Forks)](https://github.com/hunterhug/gorlock/network)
[![GitHub stars](https://img.shields.io/github/stars/hunterhug/gorlock.svg?style=social&label=Stars)](https://github.com/hunterhug/gorlock/stargazers)
[![GitHub last commit](https://img.shields.io/github/last-commit/hunterhug/gorlock.svg)](https://github.com/hunterhug/gorlock)
[![GitHub issues](https://img.shields.io/github/issues/hunterhug/gorlock.svg)](https://github.com/hunterhug/gorlock/issues)

[中文说明](/README_ZH.md)

Why use distributed lock?

1. Some program processes/services may competing for resources, they distributed deployed in different position at many machines, ensure free from side effects such dirty read/write, we should lock those resources before make logic action.
2. Some crontab task may distributed deployed, we know they will run at the same time when time reach, ensure crontab task can execute one by one, we make a task lock! 
3. Other Parallel security.

Some cases: 1.avoid repeated order pay in E-commerce business, we lock the order. 2.multi copy of crontab statistical task run at night, we lock task.

We are support:

1. Single Mode Redis: Stand-alone Redis.
2. Sentinel Mode Redis: Pseudo Cluster Redis.

## How to use

Simple：

```
go get -v github.com/hunterhug/gorlock
```

Support auto keepalive.

## Example

```go
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

	time.Sleep(10 * time.Second)

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
```
