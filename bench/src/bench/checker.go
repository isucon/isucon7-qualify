package bench

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"bench/counter"
	"bench/urlcache"
)

const IsubataAppHost = "isubata.example.com"

var (
	RedirectAttemptedError = fmt.Errorf("redirect attempted")
	RequestTimeoutError    = fmt.Errorf("リクエストがタイムアウトしました")
	UserAgent              = "isucon7q-benchmarker"
	GetTimeout             = 10 * time.Second
	PostTimeout            = 3 * time.Second
	InitializeTimeout      = 10 * time.Second
	SlowThreshold          = 1000 * time.Millisecond
	MaxCheckerRequest      = 6
	DebugMode              = false
)

var (
	checkerMtx          sync.Mutex
	checkerErrorGuard   bool
	checkerErrors       []*CheckerError
	checkerLastSlowPath string
	checkerLastSlowTime time.Time

	targetHosts     []string
	requestCount    []int
	requestCountMtx sync.Mutex

	checkerRequestCounter int32 = 0

	gCache = urlcache.NewCacheStore()
)

func SetTargetHosts(target []string) {
	checkerMtx.Lock()
	defer checkerMtx.Unlock()
	targetHosts = target
	requestCount = make([]int, len(targetHosts))
}

func GetTargetHosts() []string {
	checkerMtx.Lock()
	defer checkerMtx.Unlock()

	return targetHosts
}

func GetRandomTargetHost() string {
	checkerMtx.Lock()
	defer checkerMtx.Unlock()

	return targetHosts[rand.Intn(len(targetHosts))]
}

func decRequestCount(i int) {
	requestCountMtx.Lock()
	defer requestCountMtx.Unlock()
	requestCount[i]--
}

func getFreeHostId() int {
	requestCountMtx.Lock()
	defer requestCountMtx.Unlock()
	i := rand.Intn(len(requestCount))
	for j, cnt := range requestCount {
		if requestCount[i] > cnt {
			i = j
		}
	}
	requestCount[i]++
	return i
}

type CheckerTransport struct {
	t *http.Transport
}

func (ct *CheckerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	i := getFreeHostId()
	defer decRequestCount(i)

	host := req.URL.Host
	req.URL.Host = GetTargetHosts()[i]

	if DebugMode {
		log.Println("RT", req.Header.Get("X-Request-ID"), req.Method, req.URL.String(), req.Header)
	}

	res, err := ct.t.RoundTrip(req)
	req.URL.Host = host

	return res, err
}

var (
	transport = &CheckerTransport{
		&http.Transport{
			MaxIdleConnsPerHost: 65536,
		},
	}
)

func updateLastSlowPath(path string) {
	checkerMtx.Lock()
	defer checkerMtx.Unlock()

	checkerLastSlowPath = path
	checkerLastSlowTime = time.Now()
}

func GetLastSlowPath() (path string, t time.Time) {
	checkerMtx.Lock()
	defer checkerMtx.Unlock()

	return checkerLastSlowPath, checkerLastSlowTime
}

// 起こったら即0点にするエラー
// 表示されているべきものが表示されていない
// 表示されてはいけないものが表示されていないなど
// 負荷走行中も検証できるものに限る
type fatalError struct {
	msg string
}

func (e *fatalError) Error() string {
	return fmt.Sprint("[Fatal]", e.msg)
}

func fatalErrorf(format string, a ...interface{}) error {
	return &fatalError{fmt.Sprintf(format, a...)}
}

type CheckerError struct {
	t      time.Time
	err    error
	method string
	path   string
	query  string
}

func (e *CheckerError) Error() string {
	return fmt.Sprintf("%v %v (%v %v %v)", e.t, e.err, e.method, e.path, e.query)
}

func (e *CheckerError) IsFatal() bool {
	_, ok := e.err.(*fatalError)
	return ok
}

func (e *CheckerError) IsTimeout() bool {
	return e.err == RequestTimeoutError
}

func appendError(err *CheckerError) {
	checkerMtx.Lock()
	if !checkerErrorGuard {
		checkerErrors = append(checkerErrors, err)
	}
	checkerMtx.Unlock()
}

func GuardCheckerError(guard bool) {
	checkerMtx.Lock()
	checkerErrorGuard = guard
	checkerMtx.Unlock()
}

func GetLastCheckerError() (err error, t time.Time) {
	checkerMtx.Lock()
	defer checkerMtx.Unlock()
	if len(checkerErrors) != 0 {
		e := checkerErrors[len(checkerErrors)-1]

		err = e
		t = e.t
	}
	return
}

func GetCheckerErrors() []error {
	checkerMtx.Lock()
	var errs []error
	for _, e := range checkerErrors {
		errs = append(errs, e)
	}
	checkerMtx.Unlock()
	return errs
}

type Checker struct {
	Client *http.Client
	Cache  *urlcache.CacheStore

	chRequestToken chan int
	debugHeaders   map[string]string
}

type CheckAction struct {
	Method string
	Path   string

	ContentType string
	PostBody    io.Reader         // for "multipart/form-data"
	PostData    map[string]string // for "application/x-www-form-urlencoded"
	Headers     map[string]string

	ExpectedStatusCode int
	ExpectedLocation   *regexp.Regexp
	ExpectedHeaders    map[string]string
	Description        string
	CheckFunc          func(*http.Response, *bytes.Buffer) error

	EnableCache          bool
	SkipIfCacheAvailable bool
	DisableSlowChecking  bool
}

func NewChecker() *Checker {
	c := new(Checker)

	jar, err := cookiejar.New(&cookiejar.Options{})
	if err != nil {
		log.Fatalln(err)
	}

	c.Client = &http.Client{
		Transport: transport,
		Jar:       jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return RedirectAttemptedError
		},
	}

	c.Cache = urlcache.NewCacheStore()
	c.debugHeaders = map[string]string{}
	c.chRequestToken = make(chan int, MaxCheckerRequest)
	for i := 1; i <= MaxCheckerRequest; i++ {
		c.chRequestToken <- i
	}

	return c
}

func (c *Checker) ResetCookie() {
	jar, err := cookiejar.New(&cookiejar.Options{})
	if err != nil {
		log.Fatalln(err)
	}
	c.Client.Jar = jar
}

func (c *Checker) OnError(a *CheckAction, req *http.Request, err error) error {
	// OnFailが1つのエラーに対して2回以上呼ばれた時の対策
	if _, ok := err.(*CheckerError); ok {
		return err
	}

	var cerr *CheckerError
	if req == nil {
		cerr = &CheckerError{time.Now(), err, a.Method, a.Path, ""}
	} else {
		cerr = &CheckerError{time.Now(), err, req.Method, req.URL.Path, req.URL.Query().Encode()}
	}

	appendError(cerr)
	return cerr
}

func (c *Checker) NewRequest(method, uri string, body io.Reader) (*http.Request, error) {
	parsedURL, err := url.Parse(uri)

	if err != nil {
		return nil, err
	}

	if parsedURL.Scheme == "" {
		parsedURL.Scheme = "http"
	}

	parsedURL.Host = IsubataAppHost

	req, err := http.NewRequest(method, parsedURL.String(), body)

	if err != nil {
		return nil, err
	}

	return req, err
}

func (c *Checker) Play(ctx context.Context, a *CheckAction) error {
	if ctx.Err() != nil {
		return nil
	}

	select {
	case token := <-c.chRequestToken:
		defer func() {
			c.chRequestToken <- token
		}()
	case <-ctx.Done():
		return nil
	}

	var req *http.Request
	var err error

	if strings.ToUpper(a.Method) == "POST" {
		if a.PostBody != nil {
			req, err = c.NewRequest(a.Method, a.Path, a.PostBody)
			if req != nil {
				req.Header.Set("Content-Type", a.ContentType)
			}
		} else {
			formData := url.Values{}
			for key, val := range a.PostData {
				formData.Set(key, val)
			}
			buf := bytes.NewBufferString(formData.Encode())
			req, err = c.NewRequest(a.Method, a.Path, buf)
			if req != nil {
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			}
		}
	} else {
		req, err = c.NewRequest(a.Method, a.Path, nil)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return c.OnError(a, req, fmt.Errorf("リクエストに失敗しました (主催者に連絡してください)"))
	}

	if DebugMode {
		for k, v := range c.debugHeaders {
			req.Header.Set(k, v)
		}
		cnt := atomic.AddInt32(&checkerRequestCounter, 1)
		req.Header.Set("X-Request-ID", fmt.Sprint(cnt))
	}

	if a.EnableCache {
		if cache, found := gCache.Get(a.Path); found {
			cache.ApplyRequest(req)
		} else if cache, found := c.Cache.Get(a.Path); found {
			cache.ApplyRequest(req)
		}
	}

	req.Header.Set("User-Agent", UserAgent)
	for key, val := range a.Headers {
		req.Header.Add(key, val)
	}

	timeout := GetTimeout
	if req.Method == http.MethodPost {
		timeout = PostTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req = req.WithContext(ctx)

	tm := time.AfterFunc(SlowThreshold, func() {
		if !a.DisableSlowChecking {
			updateLastSlowPath(a.Path)
		}
	})
	res, err := c.Client.Do(req)
	tm.Stop()

	isRedirectErr := false
	if urlError, ok := err.(*url.Error); ok && urlError.Err == RedirectAttemptedError {
		isRedirectErr = true
	}

	if err != nil && !isRedirectErr {
		switch e := err.(type) {
		case net.Error:
			if e.Timeout() {
				return c.OnError(a, req, RequestTimeoutError)
			}
		}

		return c.OnError(a, req, fmt.Errorf("リクエストに失敗しました %v", err))
	}

	if res == nil {
		return c.OnError(a, req, fmt.Errorf("レスポンスが不正です"))
	}

	defer res.Body.Close()

	body := GetBuffer()
	defer PutBuffer(body)

	_, err = io.Copy(body, res.Body)
	if err == context.DeadlineExceeded {
		return c.OnError(a, req, RequestTimeoutError)
	}
	// Note. リダイレクトなどのときはbodyが既に閉じられている状態で来て closed error が返るので無視する

	if 500 <= res.StatusCode {
		return c.OnError(a, res.Request, fmt.Errorf("サーバエラーが発生しました。%s", res.Status))
	}

	if a.ExpectedStatusCode != 0 && res.StatusCode != a.ExpectedStatusCode {
		return c.OnError(a, res.Request, fmt.Errorf("Response code should be %d, got %d, data: %s", a.ExpectedStatusCode, res.StatusCode, a.PostData))
	}

	if a.ExpectedLocation != nil {
		l := res.Header["Location"]
		if len(l) != 1 {
			return c.OnError(a, res.Request, fmt.Errorf("リダイレクトURLが適切に設定されていません"))
		}
		u, err := url.Parse(l[0])
		if err != nil || !a.ExpectedLocation.MatchString(u.Path) {
			return c.OnError(a, res.Request, fmt.Errorf("リダイレクト先URLが正しくありません: expected '%s', got '%s'", a.ExpectedLocation, l[0]))
		}
	}

	if res.StatusCode == 200 && a.EnableCache {
		cache, _ := urlcache.NewURLCache(res, body)
		if cache != nil {
			c.Cache.Set(a.Path, cache)
			if cache.CacheControl.Public() {
				gCache.Set(a.Path, cache)
			}
		}
	}

	if a.CheckFunc != nil {
		if err := a.CheckFunc(res, body); err != nil {
			if a.EnableCache {
				c.Cache.Del(a.Path)
			}
			return c.OnError(a, res.Request, err)
		}
	}

	counter.IncKey(a.Method + "|" + a.Path)
	return nil
}
