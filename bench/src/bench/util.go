package bench

import (
	"bytes"
	"fmt"
	"math/rand"
	"strings"
	"sync"
)

func assert(flag bool, msgs ...interface{}) {
	if !flag {
		panic("assertion failed: " + fmt.Sprint(msgs...))
	}
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func trim(s string) string {
	return strings.TrimSpace(s)
}

var alphabet = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func RandomAlphabetString(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = alphabet[rand.Intn(len(alphabet))]
	}
	return string(b)
}

func RandomText() string {
	n := len(DataSet.Texts)
	return DataSet.Texts[rand.Intn(n)] + DataSet.Texts[rand.Intn(n)] + DataSet.Texts[rand.Intn(n)]
}

var bytesBufferPool = sync.Pool{
	New: func() interface{} {
		return new(bytes.Buffer)
	},
}

func GetBuffer() *bytes.Buffer {
	return bytesBufferPool.Get().(*bytes.Buffer)
}

func PutBuffer(buf *bytes.Buffer) {
	buf.Reset()
	bytesBufferPool.Put(buf)
}
