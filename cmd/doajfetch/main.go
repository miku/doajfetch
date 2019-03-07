// UPDATE MAR 2019: This is defunkt now, try harvesting
// http://www.doaj.org/oai.article instead.
//
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
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/sethgrid/pester"
	log "github.com/sirupsen/logrus"
	"github.com/syndtr/goleveldb/leveldb"
)

const Version = "0.4.2"

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
	maxRestartCount         = flag.Int("max-restarts", 20, "maximum number of global restarts")
	outputFile              = flag.String("o", "", "output file, necessary if global restarts are used")
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

	// LevelDB cache for API responses.
	cacheDir, err := ioutil.TempDir("", "doajfetch-tmp-")
	if err != nil {
		log.Fatal(err)
	}
	db, err := leveldb.OpenFile(cacheDir, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		db.Close()
		os.RemoveAll(cacheDir)
	}()

	// Resilient client.
	client := pester.New()
	client.Concurrency = 3
	client.MaxRetries = 12
	client.Backoff = pester.ExponentialBackoff
	client.KeepLog = false

	link := fmt.Sprintf("%s/*?pageSize=%d", strings.TrimRight(*apiurl, "/"), *batchSize)

	bw := bufio.NewWriter(os.Stdout)
	defer bw.Flush()

	var counter int64            // For progress report.
	var retryCountStatusCode int // Number of retries based on HTTP status code.
	var restartCount int         // Global restart.

	// Copy everything into a tempfile as well.
	tempFile, err := ioutil.TempFile("", "doajfetch-tmp-")
	if err != nil {
		log.Fatal(err)
	}
	defer os.Remove(tempFile.Name())
	if *verbose {
		log.Printf("tempfile at %s", tempFile.Name())
	}

Outer:
	for {
		if restartCount == *maxRestartCount {
			break
		}

		counter = 0
		retryCountStatusCode = 0

		if err := tempFile.Truncate(0); err != nil {
			log.Fatal(err)
		}

		for {
			req, err := http.NewRequest("GET", link, nil)
			if err != nil {
				log.Fatal(err)
			}
			req.Header.Add("User-Agent", *userAgent)
			if *verbose {
				log.Println(req.URL.String())
			}

			// Read either from http body or bytes from cache.
			var reader io.Reader
			key := []byte(req.URL.String())

			if data, err := db.Get(key, nil); err != nil || len(data) == 0 {
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
						// Allow a global restart.
						log.Printf("failed with HTTP %d", resp.StatusCode)
						restartCount++
						break
					}
					time.Sleep(time.Duration(retryCountStatusCode) * time.Second)
					log.Printf("failed with HTTP %d, retry #%d", resp.StatusCode, retryCountStatusCode)
					retryCountStatusCode++
					continue
				}
				reader = resp.Body
			} else {
				if *verbose {
					log.Printf("cache hit on %s", string(key))
				}
				reader = bytes.NewReader(data)
			}

			// We want both, stdout and JSON parsing.
			var buf bytes.Buffer
			tee := io.TeeReader(reader, &buf)

			if _, err := io.Copy(bw, tee); err != nil {
				log.Fatal(err)
			}
			if _, err := io.WriteString(bw, "\n"); err != nil {
				log.Fatal(err)
			}
			if _, err := tempFile.Write(buf.Bytes()); err != nil {
				log.Fatal(err)
			}
			if _, err := io.WriteString(tempFile, "\n"); err != nil {
				log.Fatal(err)
			}
			if err := db.Put(key, buf.Bytes(), nil); err != nil {
				log.Println(err)
			}

			var payload ArticlesV1
			if err := json.NewDecoder(&buf).Decode(&payload); err != nil {
				log.Fatalf("%s\n%s", buf.String(), err)
			}
			if payload.Next == "" {
				break Outer
			}
			link = payload.Next

			if *showProgress {
				log.Printf("[%d][%d/%d]", restartCount, *batchSize*counter, payload.Total)
			}

			counter++
			retryCountStatusCode = 0
			time.Sleep(*sleep)
		}
	}

	if err := tempFile.Close(); err != nil {
		log.Fatal(err)
	}
	if *outputFile != "" {
		os.Rename(tempFile.Name(), *outputFile)
	}
}
