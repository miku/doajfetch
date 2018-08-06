# doajfetch
Fetch DOAJ records.

Elasticsearch does not really work any more, use a new API:

* https://doaj.org/api/v1/docs#!/Search/get_api_v1_search_articles_search_query

```
Usage of doajfetch:
  -P    show progress
  -server string
        DOAJ URL including prefix (default "https://doaj.org/query")
  -size int
        number of results per request (default 1000)
  -sleep duration
        sleep between requests (default 2s)
  -ua string
        user agent to use (default "Mozilla/5.0 (Windows NT ...")
  -verbose
        be verbose
  -version
        show version
```
