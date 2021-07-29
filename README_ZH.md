# Redis Golang分布式锁

[![GitHub forks](https://img.shields.io/github/forks/hunterhug/gorlock.svg?style=social&label=Forks)](https://github.com/hunterhug/gorlock/network)
[![GitHub stars](https://img.shields.io/github/stars/hunterhug/gorlock.svg?style=social&label=Stars)](https://github.com/hunterhug/gorlock/stargazers)
[![GitHub last commit](https://img.shields.io/github/last-commit/hunterhug/gorlock.svg)](https://github.com/hunterhug/gorlock)
[![GitHub issues](https://img.shields.io/github/issues/hunterhug/gorlock.svg)](https://github.com/hunterhug/gorlock/issues)
[![License](https://img.shields.io/badge/license-Apache%202-4EB1BA.svg)](https://www.apache.org/licenses/LICENSE-2.0.html)

[English README](/README.md)

为什么要使用分布式锁？

1. 多个进程或服务竞争资源，而这些进程或服务又部署在多台机器，此时需要使用分布式锁，确保资源被正确地使用，否则可能出现副作用。
2. 避免重复执行某一个动作，多台机器部署的同一个服务上都有相同定时任务，而定时任务只需执行一次，此时需要使用分布式锁，确保效率。

具体业务：电商业务避免重复支付，微服务上多副本服务的夜间定时统计。

分布式锁支持：

1. 单机模式的 Redis。
2. 哨兵模式的 Redis。说明：该模式也是属于单机，Redis 是一主（Master）多从（Slave），只不过有哨兵机制，主挂掉后哨兵发现后可以将从提升到主。

单机和哨兵模式的 Redis 无法做到锁的高可用，比如 Redis 挂掉了可能导致服务不可用（单机），锁混乱（哨兵），但对可靠性要求没那么高的，我们还是可以使用单机 Redis，毕竟大多数情况都比较稳定。

对可靠性要求较高，要求高可用，官方给出了 `Redlock` 算法，利用多台主（Master）来逐一加锁，半数加锁成功就认为成功，略显繁琐，要维护多套 Redis，我建议直接使用 etcd 分布式锁。

## 如何使用

很简单，执行：

```
go get -v github.com/hunterhug/gorlock
```

本库支持无感知锁续命，解决业务处理时间过长，锁被重入的问题:

```
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
	// Lock lock resource default keepAlive depend on LockFactory
	Lock(ctx context.Context, resourceName string, lockMillSecond int) (lock *Lock, err error)
	// LockForceKeepAlive lock resource force keepAlive
	LockForceKeepAlive(ctx context.Context, resourceName string, lockMillSecond int) (lock *Lock, err error)
	// LockForceNotKeepAlive lock resource force not keepAlive
	LockForceNotKeepAlive(ctx context.Context, resourceName string, lockMillSecond int) (lock *Lock, err error)
	// UnLock unlock resource
	UnLock(ctx context.Context, lock *Lock) (isUnLock bool, err error)
	// Done asynchronous see lock whether is release
	Done(lock *Lock) chan struct{}
}
```

核心操作，就是建一个锁工厂，然后使用接口契约中的方法来生成锁，并操作这把锁。

## 例子

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

	// 1. 配置Redis
	redisHost := "127.0.0.1:6379"
	redisDb := 0
	redisPass := "hunterhug" // may redis has password
	config := gorlock.NewRedisSingleModeConfig(redisHost, redisDb, redisPass)
	pool, err := gorlock.NewRedisPool(config)
	if err != nil {
		fmt.Println("redis init err:", err.Error())
		return
	}

	// 2. 新建一个锁工厂
	lockFactory := gorlock.New(pool)

	// 设置锁重试次数和尝试等待时间，表示加锁不成功等200毫秒再尝试，总共尝试3次，设置负数表示一直尝试，会堵住
	lockFactory.SetRetryCount(3).SetRetryMillSecondDelay(200)

	// 自动续命锁
	lockFactory.SetKeepAlive(true)

	// 3. 锁住资源，资源名为myLock
	resourceName := "myLock"

	// 过期时间10秒
	expireMillSecond := 10 * 1000
	lock, err := lockFactory.Lock(context.Background(), resourceName, expireMillSecond)
	if err != nil {
		fmt.Println("lock err:", err.Error())
		return
	}

	// 锁为空，表示加锁失败
	if lock == nil {
		fmt.Println("lock err:", "can not lock")
		return
	}

	// 加锁成功
	fmt.Printf("add lock success:%#v\n", lock)

	// 可以等待锁被释放
	//<-lockFactory.Done(lock)

	time.Sleep(3 * time.Second)

	// 4. 解锁
	isUnlock, err := lockFactory.UnLock(context.Background(), lock)
	if err != nil {
		fmt.Println("lock err:", err.Error())
		return
	}

	fmt.Printf("lock unlock: %v, %#v\n", isUnlock, lock)

	// 上面已经解锁了，可以多次解锁
	isUnlock, err = lockFactory.UnLock(context.Background(), lock)
	if err != nil {
		fmt.Println("lock err:", err.Error())
		return
	}

	fmt.Println("lock unlock:", isUnlock)
}
```

结果：

```
add lock success:&gorlock.Lock{CreateMillSecondTime:1627543007573, LiveMillSecondTime:10001, LastKeepAliveMillSecondTime:0, UnlockMillSecondTime:0, ResourceName:"myLock", RandomKey:"a6f438bd9b1f4d759e818dbc78e890f4", IsUnlock:false, isUnlockChan:(chan struct {})(0xc00007a1e0), IsClose:false, OpenKeepAlive:true, closeChan:(chan struct {})(0xc00007a240), keepAliveDealCloseChan:(chan bool)(0xc00007a2a0), mutex:sync.Mutex{state:0, sema:0x0}}
2021-07-29 15:16:48.083 github.com/hunterhug/gorlock:(*redisLockFactory).keepAlive [DEBUG]: start in 2021-07-29 15:16:48.079402 +0800 CST m=+0.518137615 keepAlive myLock with random key:a6f438bd9b1f4d759e818dbc78e890f4 doing
2021-07-29 15:16:48.586 github.com/hunterhug/gorlock:(*redisLockFactory).keepAlive [DEBUG]: start in 2021-07-29 15:16:48.583449 +0800 CST m=+1.022178426 keepAlive myLock with random key:a6f438bd9b1f4d759e818dbc78e890f4 doing
2021-07-29 15:16:49.090 github.com/hunterhug/gorlock:(*redisLockFactory).keepAlive [DEBUG]: start in 2021-07-29 15:16:49.087154 +0800 CST m=+1.525877218 keepAlive myLock with random key:a6f438bd9b1f4d759e818dbc78e890f4 doing
2021-07-29 15:16:49.594 github.com/hunterhug/gorlock:(*redisLockFactory).keepAlive [DEBUG]: start in 2021-07-29 15:16:49.590883 +0800 CST m=+2.029599435 keepAlive myLock with random key:a6f438bd9b1f4d759e818dbc78e890f4 doing
2021-07-29 15:16:50.097 github.com/hunterhug/gorlock:(*redisLockFactory).keepAlive [DEBUG]: start in 2021-07-29 15:16:50.094998 +0800 CST m=+2.533708261 keepAlive myLock with random key:a6f438bd9b1f4d759e818dbc78e890f4 doing
2021-07-29 15:16:50.575 github.com/hunterhug/gorlock:(*redisLockFactory).UnLock [DEBUG]: UnLock send signal to keepAlive ask game over
2021-07-29 15:16:50.575 github.com/hunterhug/gorlock:(*redisLockFactory).keepAlive [DEBUG]: start in 2021-07-29 15:16:50.575115 +0800 CST m=+3.013818937 keepAlive myLock with random key:a6f438bd9b1f4d759e818dbc78e890f4 close
2021-07-29 15:16:50.575 github.com/hunterhug/gorlock:(*redisLockFactory).UnLock [DEBUG]: UnLock receive signal to keepAlive response keepAliveHasBeenClose=false
lock unlock: true, &gorlock.Lock{CreateMillSecondTime:1627543007573, LiveMillSecondTime:10001, LastKeepAliveMillSecondTime:1627543010097, UnlockMillSecondTime:1627543010575, ResourceName:"myLock", RandomKey:"a6f438bd9b1f4d759e818dbc78e890f4", IsUnlock:true, isUnlockChan:(chan struct {})(0xc00007a1e0), IsClose:true, OpenKeepAlive:true, closeChan:(chan struct {})(0xc00007a240), keepAliveDealCloseChan:(chan bool)(0xc00007a2a0), mutex:sync.Mutex{state:0, sema:0x0}}
lock unlock: true
```

# License

```
Copyright [2019-2021] [github.com/hunterhug]

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
```