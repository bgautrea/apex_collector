package main

import (
	"encoding/xml"
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	//"fmt"
	"os"
	"strconv"
	"strings"
 	"github.com/joho/godotenv"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)
/*
<status software="4.53_4D19" hardware="1.0">
    <hostname>BG-180</hostname>
    <serial>AC4:00123</serial>
    <timezone>-6.00</timezone>
    <date>08/12/2024 09:48:49</date>
    <power>
        <failed>07/06/2024 08:00:00</failed>
        <restored>12/27/2008 18:00:22</restored>
    </power>
    <probes>
        <probe>
            <name>Tmp</name> 
            <value>77.9 </value>
            <type>Temp</type>
        </probe>
        <probe>
            <name>pH</name> 
            <value>8.04 </value>
            <type>pH</type>
        </probe>
	</probes>
    <outlets>
        <outlet>
            <name>Skimmer</name>
            <outputID>9</outputID>
            <state>AON</state>
            <deviceID>3_3</deviceID>
        </outlet>
        <outlet>
            <name>CO2Regulator</name>
            <outputID>10</outputID>
            <state>AOF</state>
            <deviceID>3_4</deviceID>
        </outlet>
	</outlets>
</status>
*/

type Status struct {
	XMLName 	xml.Name `xml:"status"`
	Software 	string	`xml:"software,attr"`
	Hardware 	string 	`xml:"hardware,attr"`
	Hostname 	string 	`xml:"hostname"`
	Serial 		string 	`xml:"serial"`
	TZ 			string 	`xml:"timezone"`
	Date 		string 	`xml:"date"`
	Power 		[]Power `xml:"power"`
	Restored 	string 	`xml:"restored"`
	Probes		[]Probe `xml:"probes>probe"`
	Outlets		[]Outlet `xml:"outlets>outlet"`
}

type Power struct {
	XMLName  xml.Name `xml:"power"`
	Failed string `xml:"failed"`
	Restored string `xml:"restored"`
}

type Probe struct {
	XMLName xml.Name `xml:"probe"`
	Type string `xml:"type"`
	Name string `xml:"name"`
	Value string `xml:"value"`
}

type Outlet struct {
	XMLName xml.Name `xml:"outlet"`
	Name string `xml:"name"`
	OutputId string `xml:"outputID"`
	State string `xml:"state"`
	DeviceId string `xml:"deviceID"`
}

const namespace = "apex"
const apexStatusEndpoint = "/cgi-bin/status.xml"

var (
	tr = &http.Transport{}

	client = &http.Client{Transport: tr}

	listenAddress = flag.String("web.listen-address", ":9141",
		"Address to listen on for telemetry")
	metricsPath = flag.String("web.telemetry-path", "/metrics",
		"Path under which to expose metrics")
	configPath = flag.String("config.file-path", "",
		"Path to environment file")
	// Metrics
	up = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "up"),
		"Was the last Apex query successful.",
		nil, nil,
	)
	currentProbeValue = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "current_probe_value"),
		"What is the current value of the Probe (per probe).",
		[]string{"probe"}, nil,
	)
	currentOutletState = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "current_outlet_state"),
		"What is the current state of the outlet (per outlet). 0 = Off, 1 = On, 2 = Auto Off, 3 = Auto On, 4 = UNKNOWN",
		[]string{"outlet"}, nil,
	)
	foo = "bar"
)

type Exporter struct {
	apexEndpoint, apexUsername, apexPassword string
}

func NewExporter(apexEndpoint string, apexUsername string, apexPassword string) *Exporter {
	return &Exporter{
		apexEndpoint: apexEndpoint,
		apexUsername: apexUsername,
		apexPassword: apexPassword,
	}
}

func (e *Exporter) Describe(ch chan <- *prometheus.Desc) {
	ch <- up
	ch <- currentProbeValue
	ch <- currentOutletState
}

func (e *Exporter) Collect(ch chan <- prometheus.Metric) {
	e.ApexUpdateMetrics(ch)
}

func (e *Exporter) ApexUpdateMetrics(ch chan <- prometheus.Metric) {
	// Load channel stats
	req, err := http.NewRequest("GET", e.apexEndpoint+apexStatusEndpoint, nil)
	if err != nil {
		log.Fatal(err)
	}
	//If for some reason the Apex requires authentication. I havent seen it need this though.
	//req.SetBasicAuth(e.apexUsername, e.apexPassword)

	// Make request and show output.
	resp, err := client.Do(req)
	if err != nil {
		ch <- prometheus.MustNewConstMetric(
			up, prometheus.GaugeValue, 0,)
		log.Fatal(err)
	}
	ch <- prometheus.MustNewConstMetric(
		up, prometheus.GaugeValue, 1,)

	body, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		log.Fatal(err)
	}
	// fmt.Println(string(body))

	// we initialize our array
	var status Status
	// we unmarshal our byteArray which contains our
	// xmlFiles content into 'users' which we defined above
	err = xml.Unmarshal(body, &status)
	if err != nil {
		log.Fatal(err)
	}

	for i := 0; i < len(status.Probes); i++ {
		probeName := status.Probes[i].Name
		f, err := strconv.ParseFloat(strings.TrimSpace(status.Probes[i].Value), 64)

		if err != nil {
			log.Println(err)
		} else {
			ch <- prometheus.MustNewConstMetric(
				currentProbeValue, prometheus.GaugeValue, f, probeName,
			)
		}
	}

	for j := 0; j < len(status.Outlets); j++ {
		outletName := status.Outlets[j].Name
		
		//outletState, _ := status.Outlets[j].State
		if (status.Outlets[j].State == "OFF") {
			ch <- prometheus.MustNewConstMetric(
				currentOutletState, prometheus.GaugeValue, 0, outletName,
			)
		} else if ( status.Outlets[j].State == "ON" ) {
			ch <- prometheus.MustNewConstMetric(
				currentOutletState, prometheus.GaugeValue, 1, outletName,
			)
		} else if ( status.Outlets[j].State == "AON" ) {
			ch <- prometheus.MustNewConstMetric(
				currentOutletState, prometheus.GaugeValue, 2, outletName,
			)
		} else if ( status.Outlets[j].State == "AOF" ) {
			ch <- prometheus.MustNewConstMetric(
				currentOutletState, prometheus.GaugeValue, 3, outletName,
			)
		} else { 
			ch <- prometheus.MustNewConstMetric(
				currentOutletState, prometheus.GaugeValue, 4, outletName,
			)
		 }

	}

	log.Println("Endpoint scraped")
}

func main() {
	flag.Parse()

	configFile := *configPath
	if configFile != "" {
		  log.Printf("Loading %s env file.\n", configFile)
	  err := godotenv.Load(configFile)
		if err != nil {
			log.Printf("Error loading %s env file.\n", configFile)
		}
	} else {
		err := godotenv.Load()
		if err != nil {
			log.Println("Error loading .env file, assume env variables are set.")
		}
	
	//apexEndpoint := "http://192.168.0.218"
	//apexUsername := "admin"
	//apexPassword := "1234"

	apexEndpoint := os.Getenv("APEX_ENDPOINT")
	apexUsername := os.Getenv("APEX_USERNAME")
	apexPassword := os.Getenv("APEX_PASSWORD")


	exporter := NewExporter(apexEndpoint, apexUsername, apexPassword)
	prometheus.MustRegister(exporter)
	log.Printf("Using connection endpoint: %s", apexEndpoint)
  
	  http.Handle(*metricsPath, promhttp.Handler())
	  http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		  w.Write([]byte(`<html>
			   <head><title>Apex Status Exporter</title></head>
			   <body>
			   <h1>Apex Status Exporter</h1>
			   <p><a href='` + *metricsPath + `'>Metrics</a></p>
			   </body>
			   </html>`))
	  })
	  log.Fatal(http.ListenAndServe(":9141", nil))
 	}
}