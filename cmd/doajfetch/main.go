// Fetch documents from DOAJ. Data resides in an elasticsearch server, API v1:
//
//     $ curl -X GET --header "Accept: application/json" "https://doaj.org/api/v1/search/articles/*"
//
// https://doaj.org/api/v1/docs#!/Search/get_api_v1_search_articles_search_query
//
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/sethgrid/pester"
	log "github.com/sirupsen/logrus"
)

const Version = "0.4.0"

var (
	apiurl                  = flag.String("url", "https://doaj.org/api/v1/search/articles", "DOAJ API endpoint URL")
	batchSize               = flag.Int64("size", 100, "number of results per request (page)")
	userAgent               = flag.String("ua", fmt.Sprintf("doajfetch/%s", Version), "user agent string")
	verbose                 = flag.Bool("verbose", false, "be verbose")
	showProgress            = flag.Bool("P", false, "show progress")
	sleep                   = flag.Duration("sleep", 2*time.Second, "sleep between requests")
	showVersion             = flag.Bool("version", false, "show version")
	maxRetries              = flag.Int("max-retries", 10, "retry failed requests")
	maxRetriesStatusCode    = flag.Int("max-retries-status-code", 10, "retry requests with HTTP >= 400")
	maxSleepBetweenRequests = flag.Duration("max-sleep", 10*time.Second, "maximum number of seconds to sleep between requests")
)

// ArticlesV1 is returned from https://doaj.org/api/v1/search/articles/*. The
// next page URL can be found in next. On the last page next will be empty.
type ArticlesV1 struct {
	Last     string `json:"last"`
	Next     string `json:"next"`
	Page     int64  `json:"page"`
	PageSize int64  `json:"pageSize"`
	Query    string `json:"query"`
	Results  []struct {
		Bibjson     interface{} `json:"bibjson"`
		CreatedDate string      `json:"created_date"`
		Id          string      `json:"id"`
		LastUpdated string      `json:"last_updated"`
	} `json:"results"`
	Timestamp string `json:"timestamp"`
	Total     int64  `json:"total"`
}

func main() {
	flag.Parse()

	if *showVersion {
		fmt.Println(Version)
		os.Exit(0)
	}

	client := pester.New()
	client.Concurrency = 3
	client.MaxRetries = 12
	client.Backoff = pester.ExponentialBackoff
	client.KeepLog = false

	link := fmt.Sprintf("%s/*?pageSize=%d", strings.TrimRight(*apiurl, "/"), *batchSize)

	bw := bufio.NewWriter(os.Stdout)
	defer bw.Flush()

	var counter int64
	var retryCountStatusCode int // number of retries based on HTTP status code

	for {
		req, err := http.NewRequest("GET", link, nil)
		if err != nil {
			log.Fatal(err)
		}
		req.Header.Add("User-Agent", *userAgent)
		if *verbose {
			log.Println(req.URL.String())
		}
		resp, err := client.Do(req)
		if err != nil {
			log.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 400 {
			if resp.StatusCode == 429 {
				if *sleep < *maxSleepBetweenRequests {
					*sleep = *sleep * 2
					log.Printf("due to HTTP 429, increasing sleep between requests to %v", *sleep)
					time.Sleep(*sleep)
					continue
				}
			}
			if retryCountStatusCode == *maxRetriesStatusCode {
				log.Fatalf("failed with HTTP %d", resp.StatusCode)
			}
			time.Sleep(time.Duration(retryCountStatusCode) * time.Second)
			log.Println("failed with HTTP %d, retry #%d", resp.StatusCode, retryCountStatusCode)
			retryCountStatusCode++
			continue
		}

		var buf bytes.Buffer
		tee := io.TeeReader(resp.Body, &buf)

		if _, err := io.Copy(bw, tee); err != nil {
			log.Fatal(err)
		}
		if _, err := bw.Write([]byte("\n")); err != nil {
			log.Fatal(err)
		}

		var payload ArticlesV1
		if err := json.NewDecoder(&buf).Decode(&payload); err != nil {
			log.Println(buf.String())
			log.Fatal(err)
		}
		if payload.Next == "" {
			break
		}
		link = payload.Next

		if *showProgress {
			log.Printf("%d/%d", *batchSize*counter, payload.Total)
		}

		counter++
		retryCountStatusCode = 0
		time.Sleep(*sleep)
	}
}
