// Fetch documents from DOAJ.
//
// https://doaj.org/search?source=%7B%22query%22%3A%7B%22match_all%22%3A%7B%7D%7D%2C%22from%22%3A0%2C%22size%22%3A10%7D
// $ curl -v "https://doaj.org/query/journal,article" | jq .
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/sethgrid/pester"
	log "github.com/sirupsen/logrus"
)

const Version = "0.1.0"

var (
	server       = flag.String("server", "https://doaj.org/query", "DOAJ URL including prefix")
	ua           = flag.String("ua", "Mozilla/5.0 (Windows NT 6.1; WOW64; Trident/7.0; AS; rv:11.0) like Gecko", "user agent to use")
	batchSize    = flag.Int64("size", 1000, "number of results per request")
	verbose      = flag.Bool("verbose", false, "be verbose")
	showProgress = flag.Bool("P", false, "show progress")
	sleep        = flag.Duration("sleep", 2*time.Second, "sleep between requests")
	showVersion  = flag.Bool("version", false, "show version")
)

type Response struct {
	Hits struct {
		Hits []struct {
			Id     string      `json:"_id"`
			Index  string      `json:"_index"`
			Score  interface{} `json:"_score"`
			Sort   []string    `json:"sort"`
			Source interface{} `json:"_source"`
			Type   string      `json:"_type"`
		} `json:"hits"`
		MaxScore interface{} `json:"max_score"`
		Total    int64       `json:"total"`
	} `json:"hits"`
	Shards struct {
		Failed     int64 `json:"failed"`
		Successful int64 `json:"successful"`
		Total      int64 `json:"total"`
	} `json:"_shards"`
	TimedOut bool  `json:"timed_out"`
	Took     int64 `json:"took"`
}

func main() {
	flag.Parse()

	if *showVersion {
		fmt.Println(Version)
		os.Exit(0)
	}

	indices := []string{"journal", "article"}

	client := pester.New()
	client.Concurrency = 3
	client.MaxRetries = 5
	client.Backoff = pester.ExponentialBackoff
	client.KeepLog = false

	base := fmt.Sprintf("%s/%s", strings.TrimRight(*server, "/"),
		strings.Join(indices, ","))

	var from int64

	bw := bufio.NewWriter(os.Stdout)
	defer bw.Flush()

	for {
		query := fmt.Sprintf(`{"query": {"constant_score": {"query": {"match_all": {}}}}, "from": %d, "size": %d}`, from, *batchSize)

		params := url.Values{}
		params.Add("source", query)

		link := fmt.Sprintf("%s/_search?%s", base, params.Encode())

		req, err := http.NewRequest("GET", link, nil)
		if err != nil {
			log.Fatal(err)
		}
		req.Header.Add("User-Agent", *ua)

		if *verbose {
			log.Println(req.URL.String())
		}

		resp, err := client.Do(req)
		if err != nil {
			log.Fatal(err)
		}
		defer resp.Body.Close()

		var buf bytes.Buffer
		tee := io.TeeReader(resp.Body, &buf)

		if _, err := io.Copy(bw, tee); err != nil {
			log.Fatal(err)
		}
		if _, err := bw.Write([]byte("\n")); err != nil {
			log.Fatal(err)
		}

		var response Response
		if err := json.NewDecoder(&buf).Decode(&response); err != nil {
			log.Fatal(err)
		}
		if from > response.Hits.Total {
			break
		}
		from = from + *batchSize

		if *showProgress {
			log.Printf("%d/%d (%0.2f%%)", from, response.Hits.Total,
				float64(from)/float64(response.Hits.Total)*100)
		}
		time.Sleep(*sleep)
	}
}
