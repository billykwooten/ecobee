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

	// per-query descriptors
	fetchTime *prometheus.Desc

	// per-sensor descriptors
	temperature, humidity, occupancy, inUse *prometheus.Desc
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
			"temperature reported by a sensor in degrees",
			fields,
		),
		humidity: d.new(
			"humidity",
			"humidity reported by a sensor in percent",
			fields,
		),
		occupancy: d.new(
			"occupancy",
			"occupancy reported by a sensor (0 or 1)",
			fields,
		),
		inUse: d.new(
			"in_use",
			"is sensor being used in thermostat calculations (0 or 1)",
			fields,
		),
	}
}

func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.fetchTime
	ch <- c.temperature
}

func (c *Collector) Collect(ch chan<- prometheus.Metric) {
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
		for _, s := range t.RemoteSensors {
			sFields := append(tFields, s.ID, s.Name, s.Type)
			inUse := float64(0)
			if s.InUse {
				inUse = 1
			}
			ch <- prometheus.MustNewConstMetric(
				c.inUse, prometheus.GaugeValue, inUse, sFields...,
			)
			for _, sc := range s.Capability {
				switch sc.Type {
				case "temperature":
					if v, err := strconv.ParseFloat(sc.Value, 64); err == nil {
						ch <- prometheus.MustNewConstMetric(
							c.temperature, prometheus.GaugeValue, v/10, sFields...,
						)
					} else {
						glog.Error(err)
					}
				case "humidity":
					if v, err := strconv.ParseFloat(sc.Value, 64); err == nil {
						ch <- prometheus.MustNewConstMetric(
							c.humidity, prometheus.GaugeValue, v, sFields...,
						)
					} else {
						glog.Error(err)
					}
				case "occupancy":
					switch sc.Value {
					case "true":
						ch <- prometheus.MustNewConstMetric(
							c.occupancy, prometheus.GaugeValue, 1, sFields...,
						)
					case "false":
						ch <- prometheus.MustNewConstMetric(
							c.occupancy, prometheus.GaugeValue, 0, sFields...,
						)
					default:
						glog.Errorf("unknown sensor occupancy value %q", sc.Value)
					}
				default:
					glog.Infof("ignoring sensor capability %q", sc.Type)
				}
			}
		}
	}
}
