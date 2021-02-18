# Redis 分布式锁

[![GitHub forks](https://img.shields.io/github/forks/hunterhug/rlock.svg?style=social&label=Forks)](https://github.com/hunterhug/rlock/network)
[![GitHub stars](https://img.shields.io/github/stars/hunterhug/rlock.svg?style=social&label=Stars)](https://github.com/hunterhug/rlock/stargazers)
[![GitHub last commit](https://img.shields.io/github/last-commit/hunterhug/rlock.svg)](https://github.com/hunterhug/rlock)
[![Go Report Card](https://goreportcard.com/badge/github.com/hunterhug/rlock)](https://goreportcard.com/report/github.com/hunterhug/rlock)
[![GitHub issues](https://img.shields.io/github/issues/hunterhug/rlock.svg)](https://github.com/hunterhug/rlock/issues)

[English README](/README_EN.md)

为什么要使用分布式锁？

1. 多个进程或服务竞争资源，而这些进程或服务又部署在多台机器，此时需要使用分布式锁，确保资源被正确地使用，否则可能出现副作用。
2. 避免重复执行某一个动作，多台机器部署的同一个服务上都有相同定时任务，而定时任务只需执行一次，此时需要使用分布式锁，确保效率。

具体业务：电商业务避免重复支付，微服务上多副本服务的夜间定时统计。

分布式锁支持：

1. 单机模式的 Redis。
2. 哨兵模式的 Redis。

## 如何使用

很简单，执行：

```
go get -v github.com/hunterhug/rlock
```


## 例子

```go
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

```
