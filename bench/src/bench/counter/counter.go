package counter

import (
	"regexp"
	"strings"
	"sync"
)

var (
	mtx    sync.Mutex
	cntMap map[string]int64
)

func init() {
	cntMap = map[string]int64{}
}

func IncKey(key string) {
	mtx.Lock()
	cntMap[key]++
	mtx.Unlock()
}

func AddKey(key string, diff int) {
	mtx.Lock()
	cntMap[key] += int64(diff)
	mtx.Unlock()
}

func GetKey(key string) int64 {
	mtx.Lock()
	v := cntMap[key]
	mtx.Unlock()
	return v
}

func SumMatched(re *regexp.Regexp) int64 {
	var sum int64
	mtx.Lock()
	for k, v := range cntMap {
		if re.MatchString(k) {
			sum += v
		}
	}
	mtx.Unlock()
	return sum
}

func SumPrefix(prefix string) int64 {
	var sum int64
	mtx.Lock()
	for k, v := range cntMap {
		if strings.HasPrefix(k, prefix) {
			sum += v
		}
	}
	mtx.Unlock()
	return sum
}

func GetMap() map[string]int64 {
	m := map[string]int64{}
	mtx.Lock()
	for k, v := range cntMap {
		m[k] = v
	}
	mtx.Unlock()
	return m
}
