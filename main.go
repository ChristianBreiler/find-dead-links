package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"

	"github.com/PuerkitoBio/goquery"
)

type Queue struct {
	elements []string
	mu       sync.Mutex
}

func (q *Queue) enqueue(url string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.elements = append(q.elements, url)
}

func (q *Queue) dequeue() (string, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.elements) == 0 {
		return "", false
	}
	url := q.elements[0]
	q.elements = q.elements[1:]
	return url, true
}

func (q *Queue) size() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.elements)
}

type AlreadySeenSet struct {
	seenElements map[string]string
	mu           sync.Mutex
}

func (ast *AlreadySeenSet) add(url string, status string) {
	ast.mu.Lock()
	defer ast.mu.Unlock()
	ast.seenElements[url] = status
}

func (ast *AlreadySeenSet) alreadySeen(url string) bool {
	ast.mu.Lock()
	defer ast.mu.Unlock()
	_, ok := ast.seenElements[url]
	return ok
}

func (ast *AlreadySeenSet) count() int {
	ast.mu.Lock()
	defer ast.mu.Unlock()
	return len(ast.seenElements)
}

type CliOptions struct {
	output  string
	verbose bool
}

func (clio *CliOptions) init(args []string) error {
	for _, val := range args {
		switch val {
		case "--json":
			clio.output = "json"
		case "--verbose":
			clio.verbose = true
		default:
			return errors.New("Invalid commandline argument: " + val)
		}
	}
	return nil
}

func sameDomain(targetUrl, seedUrl string) bool {
	u1, err1 := url.Parse(targetUrl)
	u2, err2 := url.Parse(seedUrl)
	if err1 != nil || err2 != nil {
		return false
	}
	return u1.Hostname() == u2.Hostname()
}

func fetchUrl(targetUrl string, c chan []byte, ast *AlreadySeenSet) {
	res, err := http.Get(targetUrl)
	if err != nil {
		ast.add(targetUrl, "broken")
		c <- nil
		return
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		ast.add(targetUrl, fmt.Sprintf("status-%d", res.StatusCode))
		c <- nil
		return
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		ast.add(targetUrl, "read-error")
		c <- nil
		return
	}

	ast.add(targetUrl, "ok")
	c <- body
}

func extractHrefLinks(body []byte, baseUrl string, q *Queue, ast *AlreadySeenSet, seedUrl string) {
	base, err := url.Parse(baseUrl)
	if err != nil {
		return
	}
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return
	}

	doc.Find("a").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if !exists {
			return
		}

		if strings.HasPrefix(href, "javascript:") ||
			strings.HasPrefix(href, "mailto:") ||
			strings.HasPrefix(href, "#") {
			return
		}

		u, err := url.Parse(href)
		if err != nil {
			return
		}

		resolved := base.ResolveReference(u).String()
		if sameDomain(resolved, seedUrl) && !ast.alreadySeen(resolved) {
			q.enqueue(resolved)
		}
	})
}

func normalizeURL(urlStr string) string {
	if !strings.HasPrefix(urlStr, "http://") && !strings.HasPrefix(urlStr, "https://") {
		return "http://" + urlStr
	}
	return urlStr
}

func main() {
	if len(os.Args) < 2 {
		log.Fatalf("Usage: go run main.go <URL> [--verbose]")
	}

	seedUrl := normalizeURL(os.Args[1])
	clio := CliOptions{output: "print", verbose: false}
	if err := clio.init(os.Args[2:]); err != nil {
		log.Fatal(err)
	}

	queue := &Queue{elements: []string{}}
	alreadySeenSet := &AlreadySeenSet{seenElements: make(map[string]string)}
	c := make(chan []byte)

	if clio.output == "print" {
		fmt.Printf("Starting crawl on %s...\n", seedUrl)
	}

	go fetchUrl(seedUrl, c, alreadySeenSet)

	content := <-c
	if content == nil {
		log.Fatal("Invalid start url")
	}

	extractHrefLinks(content, seedUrl, queue, alreadySeenSet, seedUrl)

	for queue.size() > 0 && alreadySeenSet.count() < 100 {
		target, ok := queue.dequeue()
		if !ok || alreadySeenSet.alreadySeen(target) {
			continue
		}

		go fetchUrl(target, c, alreadySeenSet)
		content := <-c
		if content == nil {
			continue
		}

		if clio.output == "print" {
			fmt.Println("Fetch: " + target)
		}

		go extractHrefLinks(content, target, queue, alreadySeenSet, seedUrl)
	}

	if clio.output == "print" {
		fmt.Printf("\nDone. Checked %d URLs.\n", alreadySeenSet.count())
	}

	if clio.output == "json" {
		jsonData, err := json.MarshalIndent(alreadySeenSet.seenElements, "", "  ")
		if err != nil {
			log.Fatalf("Error serializing JSON: %s", err)
		}
		fmt.Println(string(jsonData))
	} else {
		fmt.Printf("\nDone. Checked %d URLs.\n", alreadySeenSet.count())
		for url, status := range alreadySeenSet.seenElements {
			if !clio.verbose && status == "ok" {
				continue
			}
			fmt.Printf("[%s] %s\n", status, url)
		}
	}
}
