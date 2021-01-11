package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
)

func fatalErr(err error) {
	_, _ = fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

var createFileMutex = sync.Mutex{}

func createFile(filePath string) (*os.File, error) {
	createFileMutex.Lock()
	defer createFileMutex.Unlock()

	ex := path.Ext(filePath)
	bs := filePath[:len(filePath)-len(ex)]

	for {
		filePath = bs + ex
		if _, err := os.Stat(filePath); err == nil {
			if bs[len(bs)-1] != ')' {
				bs = bs + "(1)"
				continue
			}
			l := strings.LastIndex(bs, "(")
			if l == -1 {
				bs = bs + "(1)"
				continue
			}
			i, err := strconv.Atoi(bs[l+1 : len(bs)-1])
			if err != nil {
				bs = bs + "(1)"
				continue
			}
			i++
			bs = bs[:l] + "(" + strconv.Itoa(i) + ")"
		} else {
			out, err := os.Create(filePath)
			return out, err
		}
	}

}

func downloadingFileName(ur string) (string, error) {
	u, err := url.Parse(ur)
	if err != nil {
		return "", err
	}

	ss := strings.Split(u.Path, "/")
	return ss[len(ss)-1], nil
}

func downloadFromURL(u string) {
	resp, err := http.Get(u)
	if err != nil {
		return
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	fileName, err := downloadingFileName(u)
	if err != nil {
		return
	}

	out, err := createFile(fileName)
	if err != nil {
		return
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
}

func downloadFiles(urls []string) {
	var wg sync.WaitGroup
	for _, u := range urls {
		wg.Add(1)
		go func(uu string) {
			downloadFromURL(uu)
			wg.Done()
		}(u)
	}
	wg.Wait()
}
func parseFile(p string) {
	if path.Ext(p) != ".json" {
		return
	}

	f, err := os.Open(p)
	if err != nil {
		return
	}
	defer func() {
		_ = f.Close()
	}()

	var messages []Message
	err = json.NewDecoder(f).Decode(&messages)
	if err != nil {
		return
	}

	var urls []string
	for _, m := range messages {
		if m.Type != "message" {
			continue
		}
		for _, f := range m.Files {
			urls = append(urls, f.URL)
		}
	}
	downloadFiles(urls)
}

func parseFiles(p string, files []os.FileInfo) {
	for _, f := range files {
		newP := path.Join(p, f.Name())
		if f.IsDir() {
			newFiles, err := ioutil.ReadDir(newP)
			if err != nil {
				continue
			}
			parseFiles(newP, newFiles)
		}
		parseFile(newP)
	}
}

// Message slack message type
type Message struct {
	Type  string `json:"type"`
	Files []struct {
		URL string `json:"url_private_download"`
	} `json:"files"`
}

func main() {
	dir := flag.String("f", "", "Slack export folder")
	flag.Parse()

	files, err := ioutil.ReadDir(*dir)
	if err != nil {
		fatalErr(err)
	}

	parseFiles(*dir, files)
}
