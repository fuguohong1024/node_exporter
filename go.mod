module github.com/prometheus/node_exporter

require (
	github.com/beevik/ntp v0.3.0
	github.com/containerd/containerd v1.5.2 // indirect
	github.com/coreos/go-systemd v0.0.0-20191104093116-d3cd4ed1dbcf
	github.com/docker/docker v20.10.7+incompatible
	github.com/docker/go-connections v0.4.0 // indirect
	github.com/ema/qdisc v0.0.0-20200603082823-62d0308e3e00
	github.com/go-kit/log v0.1.0
	github.com/godbus/dbus v0.0.0-20190422162347-ade71ed3457e
	github.com/hodgesds/perf-utils v0.2.5
	github.com/jsimonetti/rtnetlink v0.0.0-20210122163228-8d122574c736
	github.com/lufia/iostat v1.1.0
	github.com/mattn/go-xmlrpc v0.0.3
	github.com/mdlayher/genetlink v1.0.0 // indirect
	github.com/mdlayher/wifi v0.0.0-20200527114002-84f0b9457fdd
	github.com/mindprince/gonvml v0.0.0-20190828220739-9ebdce4bb989
	github.com/moby/term v0.0.0-20210610120745-9d4ed1856297 // indirect
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/panjf2000/ants v1.3.0
	github.com/prometheus/client_golang v1.9.0
	github.com/prometheus/client_model v0.2.0
	github.com/prometheus/common v0.26.0
	github.com/prometheus/exporter-toolkit v0.5.1
	github.com/prometheus/procfs v0.6.0
	github.com/safchain/ethtool v0.0.0-20200804214954-8f958a28363a
	github.com/siebenmann/go-kstat v0.0.0-20200303194639-4e8294f9e9d5
	github.com/soundcloud/go-runit v0.0.0-20150630195641-06ad41a06c4a
	golang.org/x/sys v0.0.0-20210324051608-47abb6519492
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
)

go 1.14
