// Package prometheus provides Prometheus support for ecobee metrics.
package prometheus

import (
	"fmt"
	"strconv"
	"time"

	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rspier/go-ecobee/ecobee"
)

type descs string

func (d descs) new(fqName, help string, variableLabels []string) *prometheus.Desc {
	return prometheus.NewDesc(fmt.Sprintf("%s_%s", d, fqName), help, variableLabels, nil)
}

// Collector implements prometheus.Collector to gather ecobee metrics on-demand.
type Collector struct {
	client *ecobee.Client

	// per-query descriptors
	fetchTime *prometheus.Desc

	// runtime descriptors
	actualTemperature, targetTemperatureMin, targetTemperatureMax *prometheus.Desc

	// sensor descriptors
	temperature, humidity, occupancy, inUse *prometheus.Desc
}

// NewCollector returns a new Collector with the given prefix assigned to all
// metrics. Note that Prometheus metrics must be unique! Don't try to create
// two Collectors with the same metric prefix.
func NewCollector(c *ecobee.Client, metricPrefix string) *Collector {
	d := descs(metricPrefix)

	// fields common across multiple metrics
	runtime := []string{"thermostat_id", "thermostat_name"}
	sensor := append(runtime, "sensor_id", "sensor_name", "sensor_type")

	return &Collector{
		client: c,

		// collector metrics
		fetchTime: d.new(
			"fetch_time",
			"elapsed time fetching data via Ecobee API",
			nil,
		),

		// thermostat (aka runtime) metrics
		actualTemperature: d.new(
			"actual_temperature",
			"thermostat-averaged current temperature",
			runtime,
		),
		targetTemperatureMax: d.new(
			"target_temperature_max",
			"maximum temperature for thermostat to maintain",
			runtime,
		),
		targetTemperatureMin: d.new(
			"target_temperature_min",
			"minimum temperature for thermostat to maintain",
			runtime,
		),

		// sensor metrics
		temperature: d.new(
			"temperature",
			"temperature reported by a sensor in degrees",
			sensor,
		),
		humidity: d.new(
			"humidity",
			"humidity reported by a sensor in percent",
			sensor,
		),
		occupancy: d.new(
			"occupancy",
			"occupancy reported by a sensor (0 or 1)",
			sensor,
		),
		inUse: d.new(
			"in_use",
			"is sensor being used in thermostat calculations (0 or 1)",
			sensor,
		),
	}
}

// Describe dumps all metric descriptors into ch.
func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.fetchTime
	ch <- c.actualTemperature
	ch <- c.targetTemperatureMax
	ch <- c.targetTemperatureMin
	ch <- c.temperature
	ch <- c.humidity
	ch <- c.occupancy
	ch <- c.inUse
}

// Collect retrieves thermostat data via the ecobee API.
func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	start := time.Now()
	tt, err := c.client.GetThermostats(ecobee.Selection{
		SelectionType:  "registered",
		IncludeSensors: true,
		IncludeRuntime: true,
	})
	elapsed := time.Now().Sub(start)
	ch <- prometheus.MustNewConstMetric(c.fetchTime, prometheus.GaugeValue, elapsed.Seconds())
	if err != nil {
		glog.Error(err)
		return
	}
	for _, t := range tt {
		tFields := []string{t.Identifier, t.Name}
		if t.Runtime.Connected {
			ch <- prometheus.MustNewConstMetric(
				c.actualTemperature, prometheus.GaugeValue, float64(t.Runtime.ActualTemperature)/10, tFields...,
			)
			ch <- prometheus.MustNewConstMetric(
				c.targetTemperatureMax, prometheus.GaugeValue, float64(t.Runtime.DesiredCool)/10, tFields...,
			)
			ch <- prometheus.MustNewConstMetric(
				c.targetTemperatureMin, prometheus.GaugeValue, float64(t.Runtime.DesiredHeat)/10, tFields...,
			)
		}
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
