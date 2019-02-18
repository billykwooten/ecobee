package main

import (
	"flag"
	"fmt"
	"net/http"
	"strconv"
	"time"

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
	flag.Parse()
	ecobee.Scopes = []string{"smartRead"}
	prometheus.MustRegister(NewCollector(ecobee.NewClient(*appID, *cacheFile), "ecobee"))
	http.Handle("/metrics", promhttp.Handler())
	glog.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *port), nil))
}

type descs string

func (d descs) new(fqName, help string, variableLabels []string) *prometheus.Desc {
	return prometheus.NewDesc(fmt.Sprintf("%s_%s", d, fqName), help, variableLabels, nil)
}

type Collector struct {
	client *ecobee.Client

	// descriptors
	fetchTime, temperature *prometheus.Desc
}

func NewCollector(c *ecobee.Client, metricPrefix string) *Collector {
	d := descs(metricPrefix)
	fields := []string{"thermostat_id", "thermostat_name", "sensor_id", "sensor_name", "sensor_type"}
	return &Collector{
		client: c,
		fetchTime: d.new(
			"fetch_time",
			"elapsed time fetching data via Ecobee API",
			nil,
		),
		temperature: d.new(
			"temperature",
			"temperature reported by a sensor",
			fields,
		),
	}
}

func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	glog.Info("desc")
	ch <- c.fetchTime
	ch <- c.temperature
}

func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	glog.Info("collect")
	start := time.Now()
	tt, err := c.client.GetThermostats(ecobee.Selection{
		SelectionType:  "registered",
		IncludeSensors: true,
	})
	elapsed := time.Now().Sub(start)
	ch <- prometheus.MustNewConstMetric(c.fetchTime, prometheus.GaugeValue, elapsed.Seconds())
	if err != nil {
		glog.Error(err)
		return
	}
	for _, t := range tt {
		tFields := []string{t.Identifier, t.Name}
		fmt.Println(t.Name, t.Identifier)
		for _, s := range t.RemoteSensors {
			fmt.Println("S", s.Name, s.ID, s.InUse)
			sFields := append(tFields, s.ID, s.Name, s.Type)
			for _, sc := range s.Capability {
				fmt.Println("SC", sc.Type, sc.Value)
				switch sc.Type {
				case "temperature":
					if v, err := strconv.ParseFloat(sc.Value, 64); err == nil {
						ch <- prometheus.MustNewConstMetric(
							c.temperature, prometheus.GaugeValue, v, sFields...,
						)
					} else {
						glog.Error(err)
					}
				default:
					glog.Infof("ignoring sensor capability %q", sc.Type)
				}
			}
		}
	}
}
