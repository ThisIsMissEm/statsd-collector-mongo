package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cactus/go-statsd-client/statsd"
	"github.com/kelseyhightower/envconfig"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

type ConfigSpecification struct {
	MongoURL          string `envconfig:"mongo_url" default:"mongodb://localhost:27017"`
	StatsdHost        string `envconfig:"statsd_host" default:"localhost"`
	StatsdPort        int    `envconfig:"statsd_port" default:"8125"`
	StatsdPrefix      string `envconfig:"statsd_prefix" default:"mongodb"`
	HostedGraphiteKey string `envconfig:"hosted_graphite_key"`
	UpdateInterval    string `envconfig:"update_interval" default:"5s"`
	Debug             bool
}

type Connections struct {
	Current      int64 `bson:"current"`
	Available    int64 `bson:"available"`
	TotalCreated int64 `bson:"totalCreated"`
}

type Mem struct {
	Resident          int64 `bson:"resident"`
	Virtual           int64 `bson:"virtual"`
	Mapped            int64 `bson:"mapped"`
	MappedWithJournal int64 `bson:"mappedWithJournal"`
}

type RWT struct {
	Readers int64 `bson:"readers"`
	Writers int64 `bson:"writers"`
	Total   int64 `bson:"total"`
}

type GlobalLock struct {
	TotalTime     int64 `bson:"totalTime"`
	LockTime      int64 `bson:"lockTime"`
	CurrentQueue  RWT   `bson:"currentQueue"`
	ActiveClients RWT   `bson:"activeClients"`
}

type Opcounters struct {
	Insert  int64 `bson:"insert"`
	Query   int64 `bson:"query"`
	Update  int64 `bson:"update"`
	Delete  int64 `bson:"delete"`
	GetMore int64 `bson:"getmore"`
	Command int64 `bson:"command"`
}

type ExtraInfo struct {
	PageFaults       int64 `bson:"page_faults"`
	HeapUsageInBytes int64 `bson:"heap_usage_bytes"`
}

type ServerStatus struct {
	Host                 string              `bson:"host"`
	Version              string              `bson:"version"`
	Process              string              `bson:"process"`
	Pid                  int64               `bson:"pid"`
	Uptime               int64               `bson:"uptime"`
	UptimeInMillis       int64               `bson:"uptimeMillis"`
	UptimeEstimate       int64               `bson:"uptimeEstimate"`
	LocalTime            bson.MongoTimestamp `bson:"localTime"`
	Connections          Connections         `bson:"connections"`
	ExtraInfo            ExtraInfo           `bson:"extra_info"`
	Mem                  Mem                 `bson:"mem"`
	GlobalLocks          GlobalLock          `bson:"globalLock"`
	Opcounters           Opcounters          `bson:"opcounters"`
	OpcountersReplicaSet Opcounters          `bson:"opcountersRepl"`
}

func serverStatus(mongoURL string) ServerStatus {
	var session *mgo.Session
	var status ServerStatus
	var err error

	session, err = mgo.DialWithTimeout(mongoURL, 30*time.Second)
	if err != nil {
		panic(err)
	}
	defer session.Close()

	// Optional. Switch the session to a monotonic behavior.
	session.SetMode(mgo.Monotonic, true)

	err = session.Run("serverStatus", &status)
	if err != nil {
		panic(err)
	}

	return status
}

func pushConnections(client statsd.Statter, connections Connections) error {
	var err error
	// Connections
	err = client.Gauge("connections.current", int64(connections.Current), 1.0)
	if err != nil {
		return err
	}

	err = client.Gauge("connections.available", int64(connections.Available), 1.0)
	if err != nil {
		return err
	}

	err = client.Gauge("connections.created", int64(connections.TotalCreated), 1.0)
	if err != nil {
		return err
	}

	return nil
}

func pushOpcounters(client statsd.Statter, opscounters Opcounters) error {
	var err error

	// Ops Counters (non-RS)
	err = client.Gauge("ops.inserts", opscounters.Insert, 1.0)
	if err != nil {
		return err
	}

	err = client.Gauge("ops.queries", opscounters.Query, 1.0)
	if err != nil {
		return err
	}

	err = client.Gauge("ops.updates", opscounters.Update, 1.0)
	if err != nil {
		return err
	}

	err = client.Gauge("ops.deletes", opscounters.Delete, 1.0)
	if err != nil {
		return err
	}

	err = client.Gauge("ops.getmores", opscounters.GetMore, 1.0)
	if err != nil {
		return err
	}

	err = client.Gauge("ops.commands", opscounters.Command, 1.0)
	if err != nil {
		return err
	}

	return nil
}

func pushMem(client statsd.Statter, mem Mem) error {
	var err error

	err = client.Gauge("mem.resident", mem.Resident, 1.0)
	if err != nil {
		return err
	}

	err = client.Gauge("mem.virtual", mem.Virtual, 1.0)
	if err != nil {
		return err
	}

	err = client.Gauge("mem.mapped", mem.Mapped, 1.0)
	if err != nil {
		return err
	}

	err = client.Gauge("mem.mapped_with_journal", mem.MappedWithJournal, 1.0)
	if err != nil {
		return err
	}

	return nil
}

func pushGlobalLocks(client statsd.Statter, glob GlobalLock) error {
	var err error

	err = client.Gauge("global_lock.total_time", glob.TotalTime, 1.0)
	if err != nil {
		return err
	}

	err = client.Gauge("global_lock.lock_time", glob.LockTime, 1.0)
	if err != nil {
		return err
	}

	err = client.Gauge("global_lock.active_readers", glob.ActiveClients.Readers, 1.0)
	if err != nil {
		return err
	}

	err = client.Gauge("global_lock.active_writers", glob.ActiveClients.Writers, 1.0)
	if err != nil {
		return err
	}

	err = client.Gauge("global_lock.active_total", glob.ActiveClients.Total, 1.0)
	if err != nil {
		return err
	}

	err = client.Gauge("global_lock.queued_readers", glob.CurrentQueue.Readers, 1.0)
	if err != nil {
		return err
	}

	err = client.Gauge("global_lock.queued_writers", glob.CurrentQueue.Writers, 1.0)
	if err != nil {
		return err
	}

	err = client.Gauge("global_lock.queued_total", glob.CurrentQueue.Total, 1.0)
	if err != nil {
		return err
	}

	return nil
}

func pushExtraInfo(client statsd.Statter, info ExtraInfo) error {
	var err error

	err = client.Gauge("extra.page_faults", info.PageFaults, 1.0)
	if err != nil {
		return err
	}

	err = client.Gauge("extra.heap_usage", info.HeapUsageInBytes, 1.0)
	if err != nil {
		return err
	}

	return nil
}

func pushStats(client statsd.Statter, status ServerStatus) error {
	var err error

	err = pushConnections(client, status.Connections)
	if err != nil {
		return err
	}

	err = pushOpcounters(client, status.Opcounters)
	if err != nil {
		return err
	}

	err = pushMem(client, status.Mem)
	if err != nil {
		return err
	}

	err = pushGlobalLocks(client, status.GlobalLocks)
	if err != nil {
		return err
	}

	err = pushExtraInfo(client, status.ExtraInfo)
	if err != nil {
		return err
	}

	return nil
}

func main() {
	var config ConfigSpecification
	var socketAddress string
	var prefix string

	err := envconfig.Process("", &config)
	if err != nil {
		log.Fatal(err.Error())
	}

	socketAddress = fmt.Sprintf("%s:%d", config.StatsdHost, config.StatsdPort)
	prefix = ""

	if len(config.HostedGraphiteKey) > 0 {
		prefix = fmt.Sprintf("%s.", config.HostedGraphiteKey)
	}

	if len(config.StatsdPrefix) > 0 {
		prefix = fmt.Sprintf("%s%s", prefix, config.StatsdPrefix)
	}

	updateInterval, err := time.ParseDuration(config.UpdateInterval)
	if err != nil {
		log.Println(fmt.Sprintf("Invalid update interval given: %s", config.UpdateInterval))
		log.Fatal(err.Error())
	}

	// Print out debug info if need be:
	if config.Debug {
		println(fmt.Sprintf("Sending stats to %s using prefix: '%s' at interval of %s", socketAddress, prefix, updateInterval))
	}

	client, err := statsd.NewClient(socketAddress, prefix)
	if err != nil {
		log.Fatal(err.Error())
	}
	defer client.Close()

	ticker := time.NewTicker(updateInterval)
	quit := make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				err := pushStats(client, serverStatus(config.MongoURL))
				if err != nil {
					fmt.Println(err)
				}
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}()

	ch := make(chan os.Signal)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	sig := <-ch
	fmt.Println("Received " + sig.String())
	close(quit)
}
