package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/kelseyhightower/envconfig"
	"github.com/prometheus/client_golang/prometheus"
)

// Specification CLI Arguments for Changing exporter behavior
type Specification struct {
	Debug         bool   `default:"false"`
	ListenAddress string `default:":9199"`
	MetricsPath   string `default:"/metrics"`
	//	Service       string `default:"sshd"`
}

var (
	service = flag.String("s", "", "A comma separated list of services you want to monitor")
	pm      = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "service_up",
		Help: "Is the service active",
	}, []string{"service"})
)

func init() {
	prometheus.MustRegister(pm)
}

func serviceCheck(s string) (float64, error) {
	//Command to check if systemd service is active
	var up float64
	cmdName := "/bin/systemctl"
	cmdArgs := []string{"is-active", s}
	cmdOut, err := exec.Command(cmdName, cmdArgs...).Output()
	isActive := strings.TrimSpace(string(cmdOut))

	up = -1
	switch isActive {
	case "inactive":
		up = 0
	case "active":
		up = 1
	case "unknown":
		up = 2
	}

	if up < 0 {
		return up, err
	}

	return up, nil
}

func main() {
	flag.Parse()
	var s Specification

	err := envconfig.Process("", &s)
	if err != nil {
		log.Fatal(err.Error())
	}

	log.Println("Starting Exporter...")
	serviceSlice := strings.Split(*service, ",")

	if *service == "" {
		fmt.Println("You need to define a service to monitor.")
	}

	go func() {
		defer func() {
			err := recover()
			if err != "" {
				log.Printf("PANIC!: %s\n", err)
			} else {
				log.Println("Finish watching.")
			}
		}()

		for {
			for i := range serviceSlice {
				x, err := serviceCheck(serviceSlice[i])
				if err != nil {
					log.Printf("Service: %s, Error: %s\n", serviceSlice[i], err)
					continue
				}
				pm.With(prometheus.Labels{"service": serviceSlice[i]}).Set(x)
			}

			time.Sleep(1 * time.Second)
		}
	}()

	log.Printf("Starting Server: %s\n", s.ListenAddress)
	log.Printf("Metrics Path: %s\n", s.MetricsPath)
	handler := prometheus.Handler()

	if s.MetricsPath == "" || s.MetricsPath == "/" {
		http.Handle(s.MetricsPath, handler)
	} else {
		http.Handle(s.MetricsPath, handler)
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`<html>
				<head><title>Prometheus Service Exporter</title></head>
				<body>
				<h1>Prometheus Service Exporter</h1>
				<p><a href="` + s.MetricsPath + `">Metrics</a></p>
				</body>
				</html>`))
		})
	}
	err = http.ListenAndServe(s.ListenAddress, nil)
	if err != nil {
		log.Fatal(err)
	}

}
