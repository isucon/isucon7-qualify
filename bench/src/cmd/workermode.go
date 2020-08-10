package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	errNoJob   = errors.New("No task")
	nodeName   = "unknown"
	pathPrefix = "fC1iWrFEw3mD7NW8KYIu5cC5DzFDGf0a/"
)

func updateNodeName() {
	name, err := os.Hostname()
	if err == nil {
		nodeName = name
	}
}

func runWorkerMode(tempDir, portalUrl string) {
	portalUrl = strings.TrimSuffix(portalUrl, "/")

	for _, arg := range os.Args {
		if strings.HasPrefix(arg, "-remotes") ||
			strings.HasPrefix(arg, "-output") {
			log.Fatalln("Cannot use the option", arg, "on workermode")
		}
	}

	updateNodeName()

	var baseArgs []string
	for _, arg := range os.Args {
		if !strings.HasPrefix(arg, "-workermode") {
			baseArgs = append(baseArgs, arg)
		}
	}

	getUrl := func(path string) (*url.URL, error) {
		u, err := url.Parse(portalUrl + path)
		if err != nil {
			return nil, err
		}
		if u.Scheme == "" {
			u.Scheme = "http"
		}
		return u, nil
	}

	getJob := func() (*Job, error) {
		u, err := getUrl("/" + pathPrefix + "job/new")
		if err != nil {
			return nil, err
		}

		res, err := http.PostForm(u.String(), url.Values{"bench_node": {nodeName}})
		if err != nil {
			return nil, err
		}
		defer res.Body.Close()

		if res.StatusCode == http.StatusNoContent {
			return nil, errNoJob
		}
		j := new(Job)
		dec := json.NewDecoder(res.Body)
		err = dec.Decode(j)
		if err != nil {
			return nil, err
		}
		return j, nil
	}

	getJobLoop := func() *Job {
		for {
			task, err := getJob()
			if err == nil {
				return task
			}

			log.Println(err)
			if err == errNoJob {
				time.Sleep(5 * time.Second)
			} else {
				time.Sleep(30 * time.Second)
			}
		}
	}

	postResult := func(job *Job, jsonPath string, logPath string, aborted bool) error {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		if file, err := os.Open(jsonPath); err == nil {
			part, _ := writer.CreateFormFile("result", filepath.Base(jsonPath))
			io.Copy(part, file)
			file.Close()
		}

		if file, err := os.Open(logPath); err == nil {
			part, _ := writer.CreateFormFile("log", filepath.Base(logPath))
			io.Copy(part, file)
			file.Close()
		}

		writer.Close()

		u, err := getUrl("/" + pathPrefix + "job/result")
		if err != nil {
			return err
		}

		q := u.Query()
		q.Set("jobid", fmt.Sprint(job.ID))
		if aborted {
			q.Set("aborted", "yes")
		}
		u.RawQuery = q.Encode()

		req, err := http.NewRequest("POST", u.String(), body)
		if err != nil {
			return err
		}

		req.Header.Set("Content-Type", writer.FormDataContentType())

		res, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}

		defer res.Body.Close()
		b, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return err
		}

		log.Println(string(b))
		return nil
	}

	for {
		job := getJobLoop()
		name := fmt.Sprintf("isucon7q-benchresult-%d-%d.json", time.Now().Unix(), job.ID)
		output := path.Join(tempDir, name)

		var args []string
		args = append(args, baseArgs...)
		args = append(args, fmt.Sprintf("-jobid=%d", job.ID))
		args = append(args, fmt.Sprintf("-remotes=%s", job.IPAddrs))
		args = append(args, fmt.Sprintf("-output=%s", output))

		ctx, cancel := context.WithCancel(context.Background())
		cmd := exec.CommandContext(ctx, args[0], args[1:]...)

		stdout, err := cmd.StdoutPipe()
		if err != nil {
			log.Println(err)
			continue
		}

		stderr, err := cmd.StderrPipe()
		if err != nil {
			log.Println(err)
			continue
		}

		log.Println("Start benchmark args:", cmd.Args)
		err = cmd.Start()
		if err != nil {
			log.Println(err)
			continue
		}

		stdoutReader := bufio.NewReader(stdout)
		stderrReader := bufio.NewReader(stderr)
		logbuf := new(bytes.Buffer)

		tm := time.AfterFunc(100*time.Second, func() {
			defer cancel()

			url := fmt.Sprintf("http://localhost:%d/debug/pprof/goroutine?debug=1", pprofPort)
			resp, err := http.Get(url)
			if err != nil {
				return
			}
			defer resp.Body.Close()
			logbuf.WriteString("--- GOROUTINEDUMP ---\n")
			io.Copy(logbuf, resp.Body)
		})

		readLog := func(r *bufio.Reader) error {
			for {
				str, err := r.ReadString('\n')
				if err != nil {
					return err
				}
				logbuf.WriteString(str)
				log.Print(str)
			}
		}

		var wg sync.WaitGroup

		wg.Add(3)
		go func() {
			defer wg.Done()
			err := readLog(stderrReader)
			if err != nil && err != io.EOF {
				log.Println(err)
			}
		}()

		go func() {
			defer wg.Done()
			err := readLog(stdoutReader)
			if err != nil && err != io.EOF {
				log.Println(err)
			}
		}()

		go func() {
			err = cmd.Wait()
			if err != nil {
				log.Println(err)
			}
			wg.Done()
		}()

		wg.Wait()
		tm.Stop()
		cancel()

		_, err = os.Stat(output)
		aborted := err != nil

		err = func() error {
			f, err := os.Create(output + ".log")
			if err != nil {
				return err
			}
			_, err = io.Copy(f, logbuf)
			if err != nil {
				return err
			}
			return nil
		}()
		if err != nil {
			log.Println(err)
		}

		// TODO:POST失敗した時のこと考える
		err = postResult(job, output, output+".log", aborted)
		if err != nil {
			log.Println(err)
		}

		time.Sleep(time.Second)
	}
}
