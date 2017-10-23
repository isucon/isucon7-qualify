package urlcache

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"net/http"
	"strings"
	"sync"

	"github.com/marcw/cachecontrol"
)

// 2017/10/20 inada-s Expireを設定したほうがリクエスト数が減ってスコアが下がる現象がみられたため、Expiresは見ないことにした。

type CacheStore struct {
	sync.Mutex
	items map[string]*URLCache
}

func NewCacheStore() *CacheStore {
	return &CacheStore{
		items: make(map[string]*URLCache),
	}
}

func (c *CacheStore) Get(key string) (*URLCache, bool) {
	c.Lock()
	v, found := c.items[key]
	c.Unlock()
	return v, found
}

func (c *CacheStore) Set(key string, value *URLCache) {
	c.Lock()
	if value == nil {
		delete(c.items, key)
	} else {
		c.items[key] = value
	}
	c.Unlock()
}

func (c *CacheStore) Del(key string) {
	c.Lock()
	delete(c.items, key)
	c.Unlock()
}

type URLCache struct {
	LastModified string
	Etag         string
	CacheControl *cachecontrol.CacheControl
	MD5          string
}

func NewURLCache(res *http.Response, body *bytes.Buffer) (*URLCache, string) {
	md5Sum := md5.Sum(body.Bytes())
	hash := hex.EncodeToString(md5Sum[:])
	ccs := res.Header["Cache-Control"]
	directive := strings.Join(ccs, " ")
	cc := cachecontrol.Parse(directive)
	noCache, _ := cc.NoCache()

	if len(directive) == 0 || noCache || cc.NoStore() {
		return nil, hash
	}

	return &URLCache{
		LastModified: res.Header.Get("Last-Modified"),
		Etag:         res.Header.Get("ETag"),
		CacheControl: &cc,
		MD5:          hash,
	}, hash
}

func (c *URLCache) ApplyRequest(req *http.Request) {
	if c.LastModified != "" {
		req.Header.Add("If-Modified-Since", c.LastModified)
	}
	if c.Etag != "" {
		req.Header.Add("If-None-Match", c.Etag)
	}
}
