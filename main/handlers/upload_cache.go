package handlers

import (
	"sync"
	"time"

	"github.com/hoshinonyaruko/gensokyo/mylog"
)

const (
	uploadCacheMaxSize    = 500
	uploadCacheSafetyTTL  = 60 // 比 API 返回的 TTL 提前 60 秒失效
	uploadCacheMinTTL     = 10
)

type uploadCacheEntry struct {
	FileInfo  string
	FileUUID  string
	ExpiresAt time.Time
}

type uploadCacheStore struct {
	mu    sync.RWMutex
	items map[string]uploadCacheEntry
}

var uploadCache = &uploadCacheStore{
	items: make(map[string]uploadCacheEntry),
}

func uploadCacheKey(contentMD5, scope, targetID string, fileType int) string {
	return contentMD5 + ":" + scope + ":" + targetID + ":" + string(rune('0'+fileType))
}

// GetCachedFileInfo 从缓存获取 file_info，未命中或已过期返回空字符串
func GetCachedFileInfo(contentMD5, scope, targetID string, fileType int) (fileInfo, fileUUID string, ok bool) {
	key := uploadCacheKey(contentMD5, scope, targetID, fileType)

	uploadCache.mu.RLock()
	entry, found := uploadCache.items[key]
	uploadCache.mu.RUnlock()

	if !found {
		return "", "", false
	}
	if time.Now().After(entry.ExpiresAt) {
		uploadCache.mu.Lock()
		delete(uploadCache.items, key)
		uploadCache.mu.Unlock()
		return "", "", false
	}
	return entry.FileInfo, entry.FileUUID, true
}

// InvalidateCachedFileInfo 清除指定缓存条目（发送失败时调用，避免复用坏的 file_info）
func InvalidateCachedFileInfo(contentMD5, scope, targetID string, fileType int) {
	key := uploadCacheKey(contentMD5, scope, targetID, fileType)
	uploadCache.mu.Lock()
	delete(uploadCache.items, key)
	uploadCache.mu.Unlock()
	mylog.Printf("[upload-cache] invalidated: key=%s (send failed)", key)
}

// SetCachedFileInfo 写入缓存。ttl 单位为秒（API 返回值）。
func SetCachedFileInfo(contentMD5, scope, targetID string, fileType int, fileInfo, fileUUID string, ttl int) {
	effectiveTTL := ttl - uploadCacheSafetyTTL
	if effectiveTTL < uploadCacheMinTTL {
		effectiveTTL = uploadCacheMinTTL
	}

	key := uploadCacheKey(contentMD5, scope, targetID, fileType)

	uploadCache.mu.Lock()
	defer uploadCache.mu.Unlock()

	if len(uploadCache.items) >= uploadCacheMaxSize {
		now := time.Now()
		for k, v := range uploadCache.items {
			if now.After(v.ExpiresAt) {
				delete(uploadCache.items, k)
			}
		}
		// 惰性清理后仍超限，删除一半
		if len(uploadCache.items) >= uploadCacheMaxSize {
			i := 0
			half := len(uploadCache.items) / 2
			for k := range uploadCache.items {
				if i >= half {
					break
				}
				delete(uploadCache.items, k)
				i++
			}
		}
	}

	uploadCache.items[key] = uploadCacheEntry{
		FileInfo:  fileInfo,
		FileUUID:  fileUUID,
		ExpiresAt: time.Now().Add(time.Duration(effectiveTTL) * time.Second),
	}
}
