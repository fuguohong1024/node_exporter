package collector

import (
	"github.com/prometheus/client_golang/prometheus"
	"strconv"
	"sync"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/mindprince/gonvml"
)

var (
	labels = []string{"minor_number", "uuid", "name"}
	isGpuServer bool = true
)

const (
	gpuNamespace = "node_gpu"
)

type gpuCollector struct {
	sync.Mutex
	numDevices  prometheus.Gauge
	usedMemory  *prometheus.GaugeVec
	totalMemory *prometheus.GaugeVec
	dutyCycle   *prometheus.GaugeVec
	powerUsage  *prometheus.GaugeVec
	temperature *prometheus.GaugeVec
	fanSpeed    *prometheus.GaugeVec
	logger log.Logger
}

func init(){
	if err := gonvml.Initialize(); err != nil {
		isGpuServer = false
	}
	defer gonvml.Shutdown()
	registerCollector("gpu",isGpuServer,newGpuCollector)
}

func newGpuCollector(logger log.Logger)(Collector, error){
	return &gpuCollector{
		numDevices: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: gpuNamespace,
				Name:      "num_devices",
				Help:      "Number of GPU devices",
			},
		),
		usedMemory: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: gpuNamespace,
				Name:      "memory_used_bytes",
				Help:      "Memory used by the GPU device in bytes",
			},
			labels,
		),
		totalMemory: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: gpuNamespace,
				Name:      "memory_total_bytes",
				Help:      "Total memory of the GPU device in bytes",
			},
			labels,
		),
		dutyCycle: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: gpuNamespace,
				Name:      "duty_cycle",
				Help:      "Percent of time over the past sample period during which one or more kernels were executing on the GPU device",
			},
			labels,
		),
		powerUsage: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: gpuNamespace,
				Name:      "power_usage_milliwatts",
				Help:      "Power usage of the GPU device in milliwatts",
			},
			labels,
		),
		temperature: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: gpuNamespace,
				Name:      "temperature_celsius",
				Help:      "Temperature of the GPU device in celsius",
			},
			labels,
		),
		fanSpeed: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: gpuNamespace,
				Name:      "fanspeed_percent",
				Help:      "Fanspeed of the GPU device as a percent of its maximum",
			},
			labels,
		),
	},nil
}


func (c *gpuCollector) Update(ch chan<- prometheus.Metric) error{
	gonvml.Initialize()
	defer gonvml.Shutdown()
	// Only one Collect call in progress at a time.
	c.Lock()
	defer c.Unlock()

	c.usedMemory.Reset()
	c.totalMemory.Reset()
	c.dutyCycle.Reset()
	c.powerUsage.Reset()
	c.temperature.Reset()
	c.fanSpeed.Reset()

	numDevices, err := gonvml.DeviceCount()
	if err != nil {
		level.Error(c.logger).Log("DeviceCount() error", err)
		return err
	} else {
		c.numDevices.Set(float64(numDevices))
		ch <- c.numDevices
	}

	for i := 0; i < int(numDevices); i++ {
		dev, err := gonvml.DeviceHandleByIndex(uint(i))
		if err != nil {
			level.Error(c.logger).Log("DeviceHandleByIndex() error", i, err)
			continue
		}

		minorNumber, err := dev.MinorNumber()
		if err != nil {
			level.Error(c.logger).Log("MinorNumber() error", err)
			continue
		}
		minor := strconv.Itoa(int(minorNumber))

		uuid, err := dev.UUID()
		if err != nil {
			level.Error(c.logger).Log("UUID() error", err)
			continue
		}

		name, err := dev.Name()
		if err != nil {
			level.Error(c.logger).Log("Name() error", err)
			continue
		}

		totalMemory, usedMemory, err := dev.MemoryInfo()
		if err != nil {
			level.Error(c.logger).Log("MemoryInfo() error", err)
		} else {
			c.usedMemory.WithLabelValues(minor, uuid, name).Set(float64(usedMemory))
			c.totalMemory.WithLabelValues(minor, uuid, name).Set(float64(totalMemory))
		}

		dutyCycle, _, err := dev.UtilizationRates()
		if err != nil {
			level.Error(c.logger).Log("UtilizationRates() error", err)
		} else {
			c.dutyCycle.WithLabelValues(minor, uuid, name).Set(float64(dutyCycle))
		}

		powerUsage, err := dev.PowerUsage()
		if err != nil {
			level.Error(c.logger).Log("PowerUsage() error", err)
		} else {
			c.powerUsage.WithLabelValues(minor, uuid, name).Set(float64(powerUsage))
		}

		temperature, err := dev.Temperature()
		if err != nil {
			level.Error(c.logger).Log("Temperature() error", err)
		} else {
			c.temperature.WithLabelValues(minor, uuid, name).Set(float64(temperature))
		}

		fanSpeed, err := dev.FanSpeed()
		if err != nil {
			level.Error(c.logger).Log("FanSpeed() error", err)
		} else {
			c.fanSpeed.WithLabelValues(minor, uuid, name).Set(float64(fanSpeed))
		}
	}
	c.usedMemory.Collect(ch)
	c.totalMemory.Collect(ch)
	c.dutyCycle.Collect(ch)
	c.powerUsage.Collect(ch)
	c.temperature.Collect(ch)
	c.fanSpeed.Collect(ch)
	return nil
}