# Redis 分布式锁

[![GitHub forks](https://img.shields.io/github/forks/hunterhug/rlock.svg?style=social&label=Forks)](https://github.com/hunterhug/rlock/network)
[![GitHub stars](https://img.shields.io/github/stars/hunterhug/rlock.svg?style=social&label=Stars)](https://github.com/hunterhug/rlock/stargazers)
[![GitHub last commit](https://img.shields.io/github/last-commit/hunterhug/rlock.svg)](https://github.com/hunterhug/rlock)
[![GitHub issues](https://img.shields.io/github/issues/hunterhug/rlock.svg)](https://github.com/hunterhug/rlock/issues)

[English README](/README_EN.md)

为什么要使用分布式锁？

1. 多个进程或服务竞争资源，而这些进程或服务又部署在多台机器，此时需要使用分布式锁，确保资源被正确地使用，否则可能出现副作用。
2. 避免重复执行某一个动作，多台机器部署的同一个服务上都有相同定时任务，而定时任务只需执行一次，此时需要使用分布式锁，确保效率。

具体业务：电商业务避免重复支付，微服务上多副本服务的夜间定时统计。

分布式锁支持：

1. 单机模式的 Redis。
2. 哨兵模式的 Redis。说明：该模式也是属于单机，Redis 是一主（Master）多从（Slave），只不过有哨兵机制，主挂掉后哨兵发现后可以将从提升到主。

单机和哨兵模式的 Redis 无法做到锁的高可用，比如 Redis 挂掉了可能导致服务不可用（单机），锁混乱（哨兵），但对可靠性要求没那么高的，我们还是可以使用单机 Redis，毕竟大多数情况都比较稳定。

对可靠性要求较高，要求高可用，官方给出了 Redlock 算法，利用多台主（Master）来逐一加锁，半数加锁成功就认为成功，略显繁琐，要维护多套 Redis，我建议直接使用 etcd 分布式锁。

## 如何使用

很简单，执行：

```
go get -v github.com/hunterhug/rlock
```


## 例子

```go
package main

import (
	"context"
	"fmt"
	"github.com/hunterhug/rlock"
	"time"
)

func main() {
	rlock.SetDebug()

	// 1. config redis
	// 1. 配置Redis
	redisHost := "127.0.0.1:6379"
	redisDb := 0
	redisPass := "root" // may redis has password
	config := rlock.NewRedisSingleModeConfig(redisHost, redisDb, redisPass)
	pool, err := rlock.NewRedisPool(config)
	if err != nil {
		fmt.Println("redis init err:", err.Error())
		return
	}

	// 2. new a lock factory
	// 2. 新建一个锁工厂
	lockFactory := rlock.New(pool)

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
