package collector

import (
	"bufio"
	"context"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/go-kit/log"
	"github.com/panjf2000/ants"
	"github.com/prometheus/client_golang/prometheus"
	"gopkg.in/alecthomas/kingpin.v2"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

func init(){
	cli, _ := client.NewClientWithOpts(client.WithAPIVersionNegotiation())
	_, err := cli.Ping(context.Background())
	if err == nil {
		dockerClient = cli
		isDockerHost = true
	}
	registerCollector(netStatsPodSubsystem,isDockerHost,NewNetstatPodCollector)
}

const (
	netStatsPodSubsystem = "netstat_pod"
)


// socket状态
const (
	podUDP         = "udp"
	podTCP         = "tcp"
	podESTABLISHED = "01"
	podSYN_SENT    = "02"
	podSYN_RCVD    = "03"
	podFIIN_WAIT_1 = "04"
	podFN_WAIT_2   = "05"
	podTIME_WAIT   = "06"
	podCLOSE_WAIT  = "07"
	podCLOSED      = "08"
	podLAST_ACK    = "09"
	podLISTEN      = "0A"
	podCLOSING     = "0B"
)

var (
	dockerClient  *client.Client
	isDockerHost  bool
	podNetstatsGroutinePoolNum = kingpin.Flag("collector.podnetstat.poolnum", "Num of groutine pool to collector for netstat_pod collector.").Default("20").Int()
)

type netstatPodCollector struct {
	logger log.Logger
}


func NewNetstatPodCollector(logger log.Logger) (Collector, error){
	return &netstatPodCollector{
		logger,
	},nil
}



func (n *netstatPodCollector) Update(ch chan<- prometheus.Metric) error {
	getPodSocketStatus(ch)
	return nil
}

// 获取pod path
func getPodSocketStatus(ch chan<- prometheus.Metric)(error) {
	ctx := context.Background()
	containers,_ :=  dockerClient.ContainerList(ctx,types.ContainerListOptions{})
	if containers == nil{
		return nil
	}
	var wg sync.WaitGroup
	// 单主机容器上限100多，20协程池
	defer ants.Release()
	pool, _ := ants.NewPool(*podNetstatsGroutinePoolNum)
	for _,container := range containers{
		wg.Add(1)
		pool.Submit(getPodTask(&wg,container,ch))
	}
	wg.Wait()
	return nil
}



func getPodTask(wg *sync.WaitGroup,container types.Container,ch chan<- prometheus.Metric) func(){
	return func() {
		defer wg.Done()
		js, _ := dockerClient.ContainerInspect(context.Background(), container.ID[:10])
		tcpresult, _ := getPodSocketStats(pidPath(js.State.Pid))
		for k, v := range tcpresult {
			ch <- prometheus.MustNewConstMetric(
				prometheus.NewDesc(
					prometheus.BuildFQName(namespace, netStatsPodSubsystem, k),
					fmt.Sprintf("pods netstats: number of %s.", k),
					nil, prometheus.Labels{"pod_name": strings.ReplaceAll(js.Name, "/", "")},
				),
				prometheus.UntypedValue, float64(v),
			)
		}
	}
}



// 获取socket状态
func getPodSocketStats(fileName string) (map[string]int, error) {
	file, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	defer file.Close()
   return parsePodSocketStats(file)

}



func parsePodSocketStats(r io.Reader) (map[string]int, error) {
	var (
		socketlist = map[string]int{}
		scanner    = bufio.NewScanner(r)
		reg        = regexp.MustCompile("^0([0-9]{1}|[A-B]{1})$")
	)
	SocketCode := make(map[string]string)
	SocketCode[podESTABLISHED] ="ESTABLISHED"
	SocketCode[podSYN_SENT] = "SYN_SENT"
	SocketCode[podSYN_RCVD] =   "SYN_RCVD"
	SocketCode[podFIIN_WAIT_1] =  "FIIN_WAIT_1"
	SocketCode[podFN_WAIT_2] =  "FN_WAIT_2"
	SocketCode[podTIME_WAIT] =  "TIME_WAIT"
	SocketCode[podCLOSE_WAIT] = "CLOSE_WAIT"
	SocketCode[podCLOSED] =  "CLOSED"
	SocketCode[podLAST_ACK] =  "LAST_ACK"
	SocketCode[podLISTEN] =  "LISTEN"
	SocketCode[podCLOSING] =  "CLOSING"

	for scanner.Scan() {
		nameParts := strings.Fields(scanner.Text())
		// 匹配十六进制1-11
		for i := 0; i < len(nameParts)/2; i++ {
			if true == reg.MatchString(nameParts[i]) {
				switch nameParts[i] {
				case podESTABLISHED:
					socketlist[SocketCode[podESTABLISHED]]++
				case podSYN_SENT:
					socketlist[SocketCode[podSYN_SENT]]++
				case podSYN_RCVD:
					socketlist[SocketCode[podSYN_RCVD]]++
				case podFIIN_WAIT_1:
					socketlist[SocketCode[podFIIN_WAIT_1]]++
				case podFN_WAIT_2:
					socketlist[SocketCode[podFN_WAIT_2]]++
				case podTIME_WAIT:
					socketlist[SocketCode[podTIME_WAIT]]++
				case podCLOSE_WAIT:
					socketlist[SocketCode[podCLOSE_WAIT]]++
				case podCLOSED:
					socketlist[SocketCode[podCLOSED]]++
				case podLAST_ACK:
					socketlist[SocketCode[podLAST_ACK]]++
				case podLISTEN:
					socketlist[SocketCode[podLISTEN]]++
				case podCLOSING:
					socketlist[SocketCode[podCLOSING]]++
				}
			}
		}
	}
	return socketlist, scanner.Err()
}

func pidPath(pid int)string{
	processid  := strconv.Itoa(pid)
	return fmt.Sprintf("/proc/%s/net/tcp",processid)
}