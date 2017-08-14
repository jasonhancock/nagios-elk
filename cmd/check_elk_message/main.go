package main

import (
	"encoding/json"
	"flag"

	"github.com/jasonhancock/go-nagios"
	"gopkg.in/olivere/elastic.v3"
)

func main() {
	fs := flag.CommandLine
	p := nagios.NewPlugin("elk-message", fs)
	p.StringFlag("es", "", "Elasticsearch URL: http://127.0.0.1:9200/")
	p.StringFlag("message", "", "Message to search for")
	p.StringFlag("from", "now-1h", "time to search from")
	p.StringFlag("hostname", "", "Hostname to search against")
	p.StringFlag("username", "", "Username for basic auth")
	p.StringFlag("password", "", "Password for basic auth")
	p.StringFlag("index", "filebeat-*", "Name of the index to query in elasticsearch")
	p.StringFlag("item-type", "log", "Name of the item type to query in elasticsearch")
	p.StringFlag("field-timestamp", "@timestamp", "Name of the timestamp field to query")
	p.StringFlag("field-hostname", "beat.hostname", "Name of the hostname field to query")
	p.StringFlag("field-message", "message", "Name of the message field to query")
	flag.Parse()

	es := p.OptRequiredString("es")

	message := p.OptRequiredString("message")
	from := p.OptRequiredString("from")
	hostname := p.OptRequiredString("hostname")
	username, _ := p.OptString("username")
	password, _ := p.OptString("password")

	index := p.OptRequiredString("index")
	itype := p.OptRequiredString("item-type")
	fieldTs := p.OptRequiredString("field-timestamp")
	fieldHostname := p.OptRequiredString("field-hostname")
	fieldMessage := p.OptRequiredString("field-message")
	pageSize := 100

	opts := []elastic.ClientOptionFunc{
		elastic.SetSniff(false),
		elastic.SetURL(es),
	}

	if username != "" && password != "" {
		opts = append(opts, elastic.SetBasicAuth(username, password))
	}

	esClient, err := elastic.NewClient(opts...)
	if err != nil {
		p.Fatal(err)
	}

	bq := elastic.NewBoolQuery().
		Must(
			elastic.NewRangeQuery(fieldTs).From(from),
			elastic.NewMatchPhraseQuery(fieldMessage, message),
			elastic.NewMatchPhraseQuery(fieldHostname, hostname),
		)

	esSearch := esClient.Search().
		Index(index).
		Type(itype).
		Query(bq).
		Sort(fieldTs, false).
		Size(pageSize)

	esResults, err := esSearch.Do()
	if err != nil {
		p.Fatal(err)
	}

	verbose, _ := p.OptBool("verbose")
	if verbose {
		for _, hit := range esResults.Hits.Hits {
			var data struct {
				Message string `json:"message"`
			}

			err := json.Unmarshal(*hit.Source, &data)
			if err != nil {
				p.Verbose(err)
				continue
			}

			p.Verbose(data.Message)
		}
	}

	code := nagios.OK
	crit, _ := p.OptThreshold("critical")
	if crit.Evaluate(float64(esResults.TotalHits())) {
		code = nagios.CRITICAL
	}

	p.Exit(code, "Total matches: %d", esResults.TotalHits())
}
