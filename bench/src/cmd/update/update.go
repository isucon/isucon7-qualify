package main

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"flag"
	"fmt"
	"go/format"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"text/template"
)

var (
	publicDir string
	benchDir  string
)

func init() {
	flag.StringVar(&publicDir, "publicdir", "../webapp/public", "path to webapp/public directory")
	flag.StringVar(&benchDir, "benchdir", "./src/bench", "path to bench/src/bench directory")
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

type TemplateArg struct {
	StaticFiles []*StaticFile
}

type StaticFile struct {
	Path string
	Size int64
	Hash string
}

const staticFileTemplate = `
package bench

type StaticFile struct {
	Path string
	Size int64
	Hash string
}

var (
	StaticFiles = []*StaticFile {
{{ range .StaticFiles }} &StaticFile { "{{ .Path }}", {{ .Size }}, "{{ .Hash }}" },
{{ end }}
	}
)

`

func prepareStaticFiles() []*StaticFile {
	var ret []*StaticFile
	err := filepath.Walk(publicDir, func(path string, info os.FileInfo, err error) error {
		must(err)
		if info.IsDir() {
			return nil
		}

		subPath := path[len(publicDir):]

		f, err := os.Open(path)
		must(err)
		defer f.Close()

		h := md5.New()
		_, err = io.Copy(h, f)
		must(err)

		hash := hex.EncodeToString(h.Sum(nil))

		ret = append(ret, &StaticFile{
			Path: subPath,
			Size: info.Size(),
			Hash: hash,
		})

		return nil
	})
	must(err)

	return ret
}

func writeStaticFileGo() {
	const saveName = "staticfile.go"
	files := prepareStaticFiles()

	t := template.Must(template.New(saveName).Parse(staticFileTemplate))

	var buf bytes.Buffer
	t.Execute(&buf, TemplateArg{
		StaticFiles: files,
	})

	fmt.Print(buf.String())

	data, err := format.Source(buf.Bytes())
	must(err)

	err = ioutil.WriteFile(path.Join(benchDir, saveName), data, 0644)
	must(err)

	log.Println("save", saveName)
}

func main() {
	flag.Parse()
	publicDir, err := filepath.Abs(publicDir)
	must(err)

	if !strings.HasSuffix(publicDir, "/public") {
		log.Fatalln("invalid publicdir path")
	}

	benchDir, err := filepath.Abs(benchDir)
	must(err)

	if !strings.HasSuffix(benchDir, "src/bench") {
		log.Fatalln("invalid benchdir path")
	}

	writeStaticFileGo()
}
