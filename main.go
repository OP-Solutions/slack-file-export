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

// createFile creates file from given path
// if filePath already exists then it adds prefix (1) in the end
// if (1) is already present too then it becomes (2) then (3) and so on
// return opened os.File must me closed after using.
func createFile(filePath string) (*os.File, error) {
	filePath = *destDir + filePath
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

// downloadingFileName determines file name for given url.
func downloadingFileName(ur string) (string, error) {
	u, err := url.Parse(ur)
	if err != nil {
		return "", err
	}

	ss := strings.Split(u.Path, "/")
	return ss[len(ss)-1], nil
}

// downloadFromURL downloads single file from url.
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
	defer func() {
		_ = out.Close()
	}()

	_, err = io.Copy(out, resp.Body)
}

// downloadFiles downloads all files from url concurrently
// finishes when all downloads are finished.
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

// parseFile parses JSON file.
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

// parseFiles iterates over directories and file and searches for
// exported slack JSON files.
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

var srcDir = flag.String("src", "", "Slack export folder")
var destDir = flag.String("dest", "", "Destination of exported files")

func main() {
	flag.Parse()

	files, err := ioutil.ReadDir(*srcDir)
	if err != nil {
		fatalErr(err)
	}

	parseFiles(*srcDir, files)
}
