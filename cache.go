package main

import (
	"crypto/md5"
	"fmt"
	"time"

	"github.com/bradfitz/gomemcache/memcache"

	ecache "github.com/dgryski/go-expirecache"
)

type bytesCache interface {
	get(k string) ([]byte, bool)
	set(k string, v []byte, expire int32)
}

type nullCache struct{}

func (_ nullCache) get(string) ([]byte, bool) { return nil, false }
func (_ nullCache) set(string, []byte, int32) {}

type expireCache struct {
	ec *ecache.Cache
}

func (ec expireCache) get(k string) ([]byte, bool) {
	v, ok := ec.ec.Get(k)

	if !ok {
		return nil, false
	}

	return v.([]byte), true
}

func (ec expireCache) set(k string, v []byte, expire int32) {
	ec.ec.Set(k, v, uint64(len(v)), expire)
}

type memcachedCache struct {
	client *memcache.Client
}

func (m *memcachedCache) get(k string) ([]byte, bool) {
	hk := fmt.Sprintf("%x", md5.Sum([]byte(k)))
	done := make(chan bool, 1)

	var err error
	var item *memcache.Item

	go func() {
		item, err = m.client.Get(hk)
		done <- true
	}()

	timeout := time.After(50 * time.Millisecond)

	select {
	case <-timeout:
		Metrics.MemcacheTimeouts.Add(1)
		return nil, false
	case <-done:
	}

	if err != nil {
		return nil, false
	}

	return item.Value, true
}

func (m *memcachedCache) set(k string, v []byte, expire int32) {
	hk := fmt.Sprintf("%x", md5.Sum([]byte(k)))
	go m.client.Set(&memcache.Item{Key: hk, Value: v, Expiration: expire})
}
