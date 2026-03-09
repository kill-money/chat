// 二开：IP 限速中间件（60 req/min per IP，登录失败锁定）
package mw

import (
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/openimsdk/chat/pkg/eerrs"
	"github.com/openimsdk/tools/apiresp"
)

// ipBucket 记录单 IP 的请求桶
type ipBucket struct {
	count    int
	windowAt time.Time
}

// failRecord 记录登录失败次数
type failRecord struct {
	count     int
	lockedAt  time.Time
	expiresAt time.Time
}

var (
	rateMu   sync.Mutex
	rateMap  = map[string]*ipBucket{}
	failMu   sync.Mutex
	failMap  = map[string]*failRecord{}
)

const (
	rateWindow   = time.Minute
	rateLimit    = 60
	failMax      = 5
	lockDuration = 5 * time.Minute
)

// RateLimitByIP 每 IP 每分钟最多 60 个请求
func RateLimitByIP(c *gin.Context) {
	ip := c.ClientIP()
	now := time.Now()

	rateMu.Lock()
	bucket, ok := rateMap[ip]
	if !ok || now.Sub(bucket.windowAt) >= rateWindow {
		rateMap[ip] = &ipBucket{count: 1, windowAt: now}
		rateMu.Unlock()
		c.Next()
		return
	}
	bucket.count++
	count := bucket.count
	rateMu.Unlock()

	if count > rateLimit {
		c.Abort()
		apiresp.GinError(c, eerrs.ErrForbidden.WrapMsg("请求过于频繁，请稍后重试"))
		return
	}
	c.Next()
}

// RecordLoginFailure 记录登录失败（key=phone/email 或 IP）
func RecordLoginFailure(key string) bool {
	failMu.Lock()
	defer failMu.Unlock()

	now := time.Now()
	rec, ok := failMap[key]
	if !ok {
		failMap[key] = &failRecord{count: 1, expiresAt: now.Add(lockDuration)}
		return false
	}
	// 如果锁已到期，重置
	if now.After(rec.expiresAt) {
		failMap[key] = &failRecord{count: 1, expiresAt: now.Add(lockDuration)}
		return false
	}
	rec.count++
	if rec.count >= failMax && rec.lockedAt.IsZero() {
		rec.lockedAt = now
	}
	return rec.count >= failMax
}

// IsLoginLocked 检查是否处于登录锁定状态
func IsLoginLocked(key string) bool {
	failMu.Lock()
	defer failMu.Unlock()

	rec, ok := failMap[key]
	if !ok {
		return false
	}
	now := time.Now()
	if now.After(rec.expiresAt) {
		delete(failMap, key)
		return false
	}
	return rec.count >= failMax
}

// ResetLoginFailure 登录成功后重置失败计数
func ResetLoginFailure(key string) {
	failMu.Lock()
	defer failMu.Unlock()
	delete(failMap, key)
}
