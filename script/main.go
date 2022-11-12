package main

import (
	"autoscale/ntnx_api_call"
	"log"
	"math"
	"os"
	"time"

	"gopkg.in/ini.v1"
)

func main() {
	// Variables
	kubeconfig := "./kubeconfig.yaml"
	counter := 0

	log.Println(symbols["START"], "NKE Autoscale is starting")

	// Getting information to connect on PC, with secrets
	var myPC = new(ntnx_api_call.Ntnx_endpoint)

	// Load configuration from ini file (that should be a secret)
	log.Println(symbols["INFO"], "Getting configuration for autoscaling...")
	cfg, err := ini.Load("config/config.ini")
	CheckErr("Fail to read config/config.ini", err)

	log.Println(symbols["OK"], "  Configuration read.")

	// De/activate SSL check for API call
	tmp := ActivateSSLCheck(cfg.Section("Main").Key("check_ssl").String() == "True")
	log.Println(symbols["NEUTRAL"], "  SSL Check", tmp)

	// Get configuration from secret
	log.Println(symbols["INFO"], "Checking for Enpoint information for configuration")

	myPC = GetPCInformation()

	log.Println(symbols["OK"], "  PC configuration collected")

	// Check PC connection
	log.Println(symbols["INFO"], "Check NKE connection")

	myPC.CallAPIJSON("PC", "GET", "/karbon/v1-beta.1/k8s/clusters", "", nil)

	log.Println(symbols["OK"], "  Ok")

	// Get Kubeconfig
	log.Println(symbols["INFO"], "Getting Kubeconfig file")

	GetKubeconfig(myPC, cfg.Section("Main").Key("nke_cluster").String(), kubeconfig)

	log.Println(symbols["OK"], "  Done")

	// Check k8s credentials
	log.Println(symbols["INFO"], "Testing k8s access & metrics")

	_ = k8s_request(kubeconfig, "podnumbers", "")

	// Check Metrics
	_ = k8s_request(kubeconfig, "testmetrics", "")

	log.Println(symbols["OK"], "  Ok")

	// Start looping
	log.Println(symbols["INFO"], "Starting monitoring of NKE cluster '"+cfg.Section("Main").Key("nke_cluster").String()+"'")
	log.Println(symbols["NOTICE"], "  Authorized nodes number : from "+cfg.Section("Main").Key("min_nodenumber").String()+" to "+cfg.Section("Main").Key("max_nodenumber").String()+".")
	log.Println(symbols["NOTICE"], "  Pooling interval : "+cfg.Section("Main").Key("poolfrequency").String()+"s")

	// We intinitly loop
	for {

		// Check if Kuebconfig has to be refreshed
		now := time.Now()
		file, err := os.Stat(kubeconfig)
		CheckErr("kubeconfig age control", err)

		if now.Sub(file.ModTime()).Hours() > 20 {
			log.Println(symbols["REFRESH"], "  Your kubeconfig is old, and is going to be refreshed...")
			GetKubeconfig(myPC, cfg.Section("Main").Key("nke_cluster").String(), kubeconfig)
		}

		// Get Load of the cluster
		Load := k8s_request(kubeconfig, "load", "")

		// Display Load
		scalerequest := float64(AnalyseClusterLoad(Load, cfg))
		if math.Abs(scalerequest) == 1 {
			counter++
		}

		// If counter reach limit, we do the scale request
		if counter >= cfg.Section("Main").Key("occurences").MustInt() {

			// Check the current number of nodes
			nodenumbers := k8s_request(kubeconfig, "nodenumbers", "")[0]

			if scalerequest == 1 {
				// New node is expected
				if int(nodenumbers) < cfg.Section("Main").Key("max_nodenumber").MustInt() {
					log.Println(symbols["INFO"], "  Scaling request to", nodenumbers+scalerequest, "nodes")
					ScaleClusterTo(myPC, cfg.Section("Main").Key("nke_cluster").MustString(""), 1, cfg.Section("Main").Key("nodepool").MustString(""), cfg.Section("Main").Key("wait_after_scaleout").MustInt())
				} else {
					log.Println(symbols["WARN"], "  New node is expected but cluster reached maximum size (", nodenumbers, " workers)")
				}
			} else if scalerequest == -1 {
				//  Node removal is expected
				if int(nodenumbers) > cfg.Section("Main").Key("min_nodenumber").MustInt() {
					log.Println(symbols["INFO"], "    Scaling request to", nodenumbers+scalerequest, "nodes")
					ScaleClusterTo(myPC, cfg.Section("Main").Key("nke_cluster").MustString(""), -1, cfg.Section("Main").Key("nodepool").MustString(""), cfg.Section("Main").Key("wait_after_scaleout").MustInt())
				}
			}

			// Reste counter
			counter = 0
		}

		// Wait for next analyse
		time.Sleep(time.Duration(cfg.Section("Main").Key("poolfrequency").MustInt()) * time.Second)
	}
}
