// Copyright 2015 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// +build !nonetstat

package collector

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"gopkg.in/alecthomas/kingpin.v2"
)

const (
	netStatsSubsystem = "netstat"
)

var (
	netStatFields = kingpin.Flag("collector.netstat.fields", "Regexp of fields to return for netstat collector.").Default("^(.*_(InErrors|InErrs)|Ip_Forwarding|Ip(6|Ext)_(InOctets|OutOctets)|Icmp6?_(InMsgs|OutMsgs)|TcpExt_(Listen.*|Syncookies.*|TCPSynRetrans)|Tcp_(ActiveOpens|InSegs|OutSegs|OutRsts|PassiveOpens|RetransSegs|CurrEstab)|Udp6?_(InDatagrams|OutDatagrams|NoPorts|RcvbufErrors|SndbufErrors))$").String()
)

type netStatCollector struct {
	fieldPattern *regexp.Regexp
	logger       log.Logger
}

func init() {
	// 注册到map中
	registerCollector("netstat", defaultEnabled, NewNetStatCollector)
}

// NewNetStatCollector takes and returns
// a new Collector exposing network stats.
func NewNetStatCollector(logger log.Logger) (Collector, error) {
	pattern := regexp.MustCompile(*netStatFields)
	return &netStatCollector{
		fieldPattern: pattern,
		logger:       logger,
	}, nil
}

func (c *netStatCollector) Update(ch chan<- prometheus.Metric) error {
	netStats, err := getNetStats(procFilePath("net/netstat"))
	if err != nil {
		return fmt.Errorf("couldn't get netstats: %w", err)
	}
	snmpStats, err := getNetStats(procFilePath("net/snmp"))
	if err != nil {
		return fmt.Errorf("couldn't get SNMP stats: %w", err)
	}
	snmp6Stats, err := getSNMP6Stats(procFilePath("net/snmp6"))
	if err != nil {
		return fmt.Errorf("couldn't get SNMP6 stats: %w", err)
	}
	tcpstats, err := getSocketStats(procFilePath("net/tcp"))
	if err != nil {
		return fmt.Errorf("couldn't get TCP stats: %w", err)
	}
	udpstats, err := getSocketStats(procFilePath("net/udp"))
	if err != nil {
		return fmt.Errorf("couldn't get UDP stats: %w", err)
	}
	// Merge the results of snmpStats into netStats (collisions are possible, but
	// we know that the keys are always unique for the given use case).
	for k, v := range snmpStats {
		netStats[k] = v
	}
	for k, v := range snmp6Stats {
		netStats[k] = v
	}
	for protocol, protocolStats := range netStats {
		for name, value := range protocolStats {
			key := protocol + "_" + name
			v, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return fmt.Errorf("invalid value %s in netstats: %w", value, err)
			}
			if !c.fieldPattern.MatchString(key) {
				continue
			}
			ch <- prometheus.MustNewConstMetric(
				prometheus.NewDesc(
					prometheus.BuildFQName(namespace, netStatsSubsystem, key),
					fmt.Sprintf("Statistic %s.", protocol+name),
					nil, nil,
				),
				prometheus.UntypedValue, v,
			)
		}
	}
	// TCP_status
	for k, v := range tcpstats {
		ch <- prometheus.MustNewConstMetric(
			prometheus.NewDesc(
				prometheus.BuildFQName(namespace, netStatsSubsystem, k),
				fmt.Sprintf("number of %s.", k),
				nil, nil,
			),
			prometheus.UntypedValue, float64(v),
		)
	}
	// UDP_status
	for k, v := range udpstats {
		ch <- prometheus.MustNewConstMetric(
			prometheus.NewDesc(
				prometheus.BuildFQName(namespace, netStatsSubsystem, k),
				fmt.Sprintf("number of %s.", k),
				nil, nil,
			),
			prometheus.UntypedValue, float64(v),
		)
	}
	return nil
}

func getNetStats(fileName string) (map[string]map[string]string, error) {
	file, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return parseNetStats(file, fileName)
}

func parseNetStats(r io.Reader, fileName string) (map[string]map[string]string, error) {
	var (
		netStats = map[string]map[string]string{}
		scanner  = bufio.NewScanner(r)
	)

	for scanner.Scan() {
		nameParts := strings.Split(scanner.Text(), " ")
		scanner.Scan()
		valueParts := strings.Split(scanner.Text(), " ")
		// Remove trailing :.
		protocol := nameParts[0][:len(nameParts[0])-1]
		netStats[protocol] = map[string]string{}
		if len(nameParts) != len(valueParts) {
			return nil, fmt.Errorf("mismatch field count mismatch in %s: %s",
				fileName, protocol)
		}
		for i := 1; i < len(nameParts); i++ {
			netStats[protocol][nameParts[i]] = valueParts[i]
		}
	}

	return netStats, scanner.Err()
}

func getSNMP6Stats(fileName string) (map[string]map[string]string, error) {
	file, err := os.Open(fileName)
	if err != nil {
		// On systems with IPv6 disabled, this file won't exist.
		// Do nothing.
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}

		return nil, err
	}
	defer file.Close()

	return parseSNMP6Stats(file)
}

func parseSNMP6Stats(r io.Reader) (map[string]map[string]string, error) {
	var (
		netStats = map[string]map[string]string{}
		scanner  = bufio.NewScanner(r)
	)

	for scanner.Scan() {
		stat := strings.Fields(scanner.Text())
		if len(stat) < 2 {
			continue
		}
		// Expect to have "6" in metric name, skip line otherwise
		if sixIndex := strings.Index(stat[0], "6"); sixIndex != -1 {
			protocol := stat[0][:sixIndex+1]
			name := stat[0][sixIndex+1:]
			if _, present := netStats[protocol]; !present {
				netStats[protocol] = map[string]string{}
			}
			netStats[protocol][name] = stat[1]
		}
	}

	return netStats, scanner.Err()
}

// socket状态
const (
	UDP         = "udp"
	TCP         = "tcp"
	ESTABLISHED = "01"
	SYN_SENT    = "02"
	SYN_RCVD    = "03"
	FIIN_WAIT_1 = "04"
	FN_WAIT_2   = "05"
	TIME_WAIT   = "06"
	CLOSE_WAIT  = "07"
	CLOSED      = "08"
	LAST_ACK    = "09"
	LISTEN      = "0A"
	CLOSING     = "0B"
)

// 获取socket状态
func getSocketStats(fileName string) (map[string]int, error) {
	file, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	boolv, _ := regexp.MatchString(TCP, fileName)
	switch boolv {
	case true:
		return parseSocketStats(file, TCP)
	default:
		return parseSocketStats(file, UDP)
	}
}

func parseSocketStats(r io.Reader, s string) (map[string]int, error) {
	var (
		socketlist = map[string]int{}
		scanner    = bufio.NewScanner(r)
		reg        = regexp.MustCompile("^0([0-9]{1}|[A-B]{1})$")
	)
	socket := strings.ToUpper(s)
	SocketCode := make(map[string]string)
	SocketCode[ESTABLISHED] = socket + "_ESTABLISHED"
	SocketCode[SYN_SENT] = socket + "_SYN_SENT"
	SocketCode[SYN_RCVD] = socket + "_SYN_RCVD"
	SocketCode[FIIN_WAIT_1] = socket + "_FIIN_WAIT_1"
	SocketCode[FN_WAIT_2] = socket + "_FN_WAIT_2"
	SocketCode[TIME_WAIT] = socket + "_TIME_WAIT"
	SocketCode[CLOSE_WAIT] = socket + "_CLOSE_WAIT"
	SocketCode[CLOSED] = socket + "_CLOSED"
	SocketCode[LAST_ACK] = socket + "_LAST_ACK"
	SocketCode[LISTEN] = socket + "_LISTEN"
	SocketCode[CLOSING] = socket + "_CLOSING"

	for scanner.Scan() {
		nameParts := strings.Fields(scanner.Text())
		// 匹配十六进制1-11
		for i := 0; i < len(nameParts)/2; i++ {
			if true == reg.MatchString(nameParts[i]) {
				switch nameParts[i] {
				case ESTABLISHED:
					socketlist[SocketCode[ESTABLISHED]]++
				case SYN_SENT:
					socketlist[SocketCode[SYN_SENT]]++
				case SYN_RCVD:
					socketlist[SocketCode[SYN_RCVD]]++
				case FIIN_WAIT_1:
					socketlist[SocketCode[FIIN_WAIT_1]]++
				case FN_WAIT_2:
					socketlist[SocketCode[FN_WAIT_2]]++
				case TIME_WAIT:
					socketlist[SocketCode[TIME_WAIT]]++
				case CLOSE_WAIT:
					socketlist[SocketCode[CLOSE_WAIT]]++
				case CLOSED:
					socketlist[SocketCode[CLOSED]]++
				case LAST_ACK:
					socketlist[SocketCode[LAST_ACK]]++
				case LISTEN:
					socketlist[SocketCode[LISTEN]]++
				case CLOSING:
					socketlist[SocketCode[CLOSING]]++
				}
			}
		}
	}
	return socketlist, scanner.Err()
}
