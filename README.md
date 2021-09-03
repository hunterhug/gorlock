# Redis Distributed Lock By Golang

[![GitHub forks](https://img.shields.io/github/forks/hunterhug/gorlock.svg?style=social&label=Forks)](https://github.com/hunterhug/gorlock/network)
[![GitHub stars](https://img.shields.io/github/stars/hunterhug/gorlock.svg?style=social&label=Stars)](https://github.com/hunterhug/gorlock/stargazers)
[![GitHub last commit](https://img.shields.io/github/last-commit/hunterhug/gorlock.svg)](https://github.com/hunterhug/gorlock)
[![GitHub issues](https://img.shields.io/github/issues/hunterhug/gorlock.svg)](https://github.com/hunterhug/gorlock/issues)
[![License](https://img.shields.io/badge/license-Apache%202-4EB1BA.svg)](https://www.apache.org/licenses/LICENSE-2.0.html)

[中文说明](README_ZH.md)

Why use distributed lock?

1. Some program processes/services may compete for resources, they distributed deployed in different position at many machines, ensure free from side effects such dirty read/write, we should lock those resources before make logic action.
2. Some crontab task may distribute deployed, we know they will run at the same time when time reach, ensure crontab task can execute one by one, we make a task lock! 
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

Support auto keepAlive:

```go
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
	Done(lock *Lock) chan error
}
```

If you keepAlive the lock, you must call `defer UnLock()` every time you make a Lock.

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
	redisHost := "127.0.0.1:6379"
	redisDb := 0
	redisPass := "hunterhug" // may redis has password
	config := gorlock.NewRedisSingleModeConfig(redisHost, redisDb, redisPass)
	pool, err := gorlock.NewRedisPool(config)
	if err != nil {
		fmt.Println("redis init err:", err.Error())
		return
	}

	// 2. new a lock factory
	lockFactory := gorlock.New(pool)

	// set retry time and delay mill second
	lockFactory.SetRetryCount(3).SetRetryMillSecondDelay(200)

	// keep alive will auto extend the expire time of lock
	lockFactory.SetKeepAlive(true)

	// 3. lock a resource
	resourceName := "myLock"

	// 10s
	expireMillSecond := 10 * 1000
	lock, err := lockFactory.Lock(context.Background(), resourceName, expireMillSecond)
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

	// wait lock release
	//<-lockFactory.Done(lock)

	time.Sleep(3 * time.Second)

	// 4. unlock a resource
	isUnlock, err := lockFactory.UnLock(context.Background(), lock)
	if err != nil {
		fmt.Println("lock err:", err.Error())
		return
	}

	fmt.Printf("lock unlock: %v, %#v\n", isUnlock, lock)

	// unlock a resource again no effect side
	isUnlock, err = lockFactory.UnLock(context.Background(), lock)
	if err != nil {
		fmt.Println("lock err:", err.Error())
		return
	}

	fmt.Println("lock unlock:", isUnlock)
}
```

Result：

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