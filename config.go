package main

import (
	"flag"
	"fmt"
	"time"

	"github.com/vharitonsky/iniflags"
)

type strings []string

type Mongo struct {
	URL string
}

type Statsd struct {
	Host    string
	Port    int
	Env     string
	Cluster string
}

type Config struct {
	Interval time.Duration
	Mongo    Mongo
	Statsd   Statsd
}

func (s *strings) String() string {
	return fmt.Sprintf("%s", *s)
}

func (s *strings) Set(value string) error {
	*s = append(*s, value)
	return nil
}

var mongo_addresses strings

func LoadConfig() Config {
	var (
		mongo_url      = flag.String("mongo_url", "mongodb://localhost:27017", "URL to the mongodb server to gather metrics from (mongodb://...)")
		statsd_host    = flag.String("statsd_host", "localhost", "StatsD Host")
		statsd_port    = flag.Int("statsd_port", 8125, "StatsD Port")
		statsd_env     = flag.String("statsd_env", "dev", "StatsD metric environment prefix")
		statsd_cluster = flag.String("statsd_cluster", "0", "StatsD metric cluster prefix")
		interval       = flag.Duration("interval", 5*time.Second, "Polling interval")
	)

	iniflags.Parse()

	cfg := Config{
		Interval: *interval,
		Mongo: Mongo{
			URL: *mongo_url,
		},
		Statsd: Statsd{
			Host:    *statsd_host,
			Port:    *statsd_port,
			Env:     *statsd_env,
			Cluster: *statsd_cluster,
		},
	}

	return cfg
}
