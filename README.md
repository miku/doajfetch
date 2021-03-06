# doajfetch

Fetch all DOAJ records, via API.

DOAJ dumps will probably be made available in Fall 2018.

> In addition, this persistent grab of our data constituted a feature request –
> it should be easier to download our entire DOAJ dataset and we will be
> implementing that over the coming weeks.

* https://blog.doaj.org/2018/10/01/infrastructure-and-why-sustainable-funding-so-important-to-services-like-doaj/

API docs:

* https://doaj.org/api/v1/docs#!/Search/get_api_v1_search_articles_search_query

Install via `go get` or [releases](https://github.com/miku/doajfetch/releases).

Note: As of August 2018, DOAJ is working on providing data dumps as well.

```
Usage of doajfetch:
  -P    show progress
  -max-retries int
        retry failed requests (default 10)
  -max-retries-status-code int
        retry requests with HTTP >= 400 (default 10)
  -max-sleep duration
        maximum number of seconds to sleep between requests (default 10s)
  -size int
        number of results per request (page) (default 100)
  -sleep duration
        sleep between requests (default 2s)
  -ua string
        user agent string (default "doajfetch/0.4.0")
  -url string
        DOAJ API endpoint URL (default "https://doaj.org/api/v1/search/articles")
  -verbose
        be verbose
  -version
        show version
```
