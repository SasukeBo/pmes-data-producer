package cache

import (
	"errors"
	"fmt"
	"github.com/SasukeBo/configer"
	"github.com/astaxie/beego/cache"
	"time"
)

var (
	globalCache cache.Cache
	expiredTime = configer.GetInt("cache_expired_time")
)

func init() {
	var err error
	globalCache, err = cache.NewCache("memory", `{"interval":60}`)
	if err != nil {
		panic(fmt.Sprintf("initial global cache failed: %v", err))
	}
}

// Set cache
func Set(key string, value interface{}) error {
	return globalCache.Put(key, value, time.Duration(expiredTime)*time.Second)
}

// Get interface value
func Get(key string) interface{} {
	return globalCache.Get(key)
}

// GetString string value
func GetString(key string) (string, error) {
	value := globalCache.Get(key)
	str, ok := value.(string)
	if !ok {
		return "", errors.New("value is not a string")
	}

	return str, nil
}

// GetBool string value
func GetBool(key string) (bool, error) {
	value := globalCache.Get(key)
	bo, ok := value.(bool)
	if !ok {
		return false, errors.New("value is not a bool")
	}

	return bo, nil
}

// FlushCacheWithKey a key value from global cache
func FlushCacheWithKey(key string) error {
	return globalCache.Delete(key)
}
