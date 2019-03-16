package main

import (
	"flag"
	"fmt"
	"net/http"

	collector "github.com/dichro/ecobee/prometheus"
	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rspier/go-ecobee/ecobee"
)

var (
	appID     = flag.String("app_id", "", "application ID from Ecobee developer portal")
	cacheFile = flag.String("cache_file", "", "auth cookie cache file")
	port      = flag.Int("port", 0, "port to serve metrics on")
)

func main() {
	defer glog.Flush()
	flag.Parse()
	ecobee.Scopes = []string{"smartRead"}
	prometheus.MustRegister(collector.NewCollector(ecobee.NewClient(*appID, *cacheFile), "ecobee"))
	http.Handle("/metrics", promhttp.Handler())
	glog.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *port), nil))
}
