package main

import (
	"evolve/debugger"
	"evolve/wikipedia/history/compressor"
	"evolve/wikipedia/history/preprocessor"
	"evolve/wikipedia/history/scraper"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
)

func main() {
	flag.Parse()
	args := flag.Args()

	flowChan := make(chan os.Signal, 1)
	signal.Notify(flowChan, syscall.SIGINT, syscall.SIGTERM)

	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	title := "Machine learning"

	debugger, err := debugger.NewDebugger()
	if err != nil {
		panic(err)
	}

	switch args[0] {
	case "scrape":
		scraper, err := scraper.NewWikiScrape(title, filepath.Join(wd, "dump", "wikipedia"), debugger)
		if err != nil {
			panic(err)
		}
		if err := scraper.Run(); err != nil {
			panic(err)
		}
		<-flowChan
		fmt.Println("Stopping the scraper")
		if err := scraper.Stop(); err != nil {
			panic(err)
		}
		scraper.PrintMetrics()
	case "process":
		dumpDir := filepath.Join(wd, "dump", "wikipedia", title)
		preprocessor, err := preprocessor.NewWikiPreprocessor(filepath.Join(wd, "dump", "wikipedia", title, "0ids.json"), nil, dumpDir, debugger)
		if err != nil {
			panic(err)
		}
		if err := preprocessor.Run(); err != nil {
			panic(err)
		}
		<-flowChan
		fmt.Println("Stopping the preprocess")
		if err := preprocessor.Stop(); err != nil {
			panic(err)
		}
		preprocessor.PrintMetrics()

	case "compress":
		dumpDir := filepath.Join(wd, "dump", "wikipedia", title)
		compressor := compressor.NewCompressor(dumpDir)
		if err := compressor.Run(); err != nil {
			panic(err)
		}
		<-flowChan
		fmt.Println("Stopping the compressor")
	}
}

/*

Base API
	https://en.wikipedia.org/w/api.php

*/
/*

https://en.wikipedia.org/w/api.php?action=query&format=json&formatversion=2&redirects=1
| Param             | What it does      | Why you care                   |
| ----------------- | ----------------- | ------------------------------ |
| `action=query`    | Core read action  | Mandatory                      |
| `format=json`     | JSON output       | Obvious                        |
| `formatversion=2` | Cleaner JSON      | **Use this always**            |
| `titles=`         | Page title(s)     | Comma-separated                |
| `redirects=1`     | Resolve redirects | `AI → Artificial intelligence` |
| `origin=*`        | CORS (browser)    | Skip for Go                    |

*/

/*

https://en.wikipedia.org/w/api.php?action=query&format=json&formatversion=2&redirects=1&titles=AI
prop=extracts&explaintext=1&exintro=1&exsectionformat=plain&exsentences=N&exlimit=1
| Param                   | Meaning           | Use case      |
| ----------------------- | ----------------- | ------------- |
| `prop=extracts`         | Page extracts     | Required      |
| `explaintext=1`         | Plain text        | LLM-friendly  |
| `exintro=1`             | Intro only        | Summaries     |
| `exsectionformat=plain` | Sectioned text    | Keeps headers |
| `exsentences=N`         | First N sentences | Fast previews |
| `exlimit=1`             | One page          | Safety        |

*/

/*
prop=revisions&rvslots=main&rvprop=content
| Param                 | Meaning             | Why           |          |            |
| --------------------- | ------------------- | ------------- | -------- | ---------- |
| `prop=revisions`      | Revision data       | Required      |          |            |
| `rvslots=main`        | Modern content slot | **Mandatory** |          |            |
| `rvprop=content`      | Wikitext            | Raw           |          |            |
| `rvprop=ids           | timestamp           | user`         | Metadata | Versioning |
| `rvlimit=1`           | Latest revision     | Default       |          |            |
| `rvstart=` / `rvend=` | Time-bounded        | Diffs         |          |            |

*/

/*
prop=links&pllimit=max&plnamespace=0
| Param           | Meaning        |
| --------------- | -------------- |
| `prop=links`    | Outgoing links |
| `pllimit=max`   | Max batch      |
| `plnamespace=0` | Articles only  |
| `plcontinue=`   | Pagination     |

*/

/*
list=search&srsearch=AI&srlimit=10
| Param           | Meaning          |       |
| --------------- | ---------------- | ----- |
| `list=search`   | Full-text search |       |
| `srsearch=`     | Query            |       |
| `srlimit=`      | Results          |       |
| `srnamespace=0` | Articles         |       |
| `srwhat=text    | title`           | Scope |


https://en.wikipedia.org/w/api.php?action=query&revids=1331771166&prop=revisions&rvslots=main&rvprop=content&format=json&formatversion=2
*/
