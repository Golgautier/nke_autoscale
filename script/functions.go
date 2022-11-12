package main

import (
	"autoscale/ntnx_api_call"
	"context"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	// "github.com/davecgh/go-spew/spew"

	"gopkg.in/ini.v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	metricsv "k8s.io/metrics/pkg/client/clientset/versioned"
)

// Emoji symbols from http://www.unicode.org/emoji/charts/emoji-list.html
var symbols = map[string]string{
	"FAIL":    "\U0000274C",
	"INFO":    "\U0001F449",
	"OK":      "\U00002705",
	"WAIT":    "\U0001F55B",
	"NEUTRAL": "\U00002796",
	"START":   "\U0001F3C1",
	"WARN":    "\U00002757",
	"REFRESH": "\U00002755",
	"NOTICE":  "\U00002755",
}

// Color codes for terminal output
var colors = map[string]string{
	"GREEN":  "\033[32m",
	"ORANGE": "\033[33m",
	"RED":    "\033[31m",
	"BLUE":   "\033[36m",
	"END":    "\033[0m",
}

// =========== CheckErr ===========
// This function is will handle errors
func CheckErr(context string, err error) {
	if err != nil {
		log.Fatal(symbols["FAIL"], " ", context, " : ", err.Error())
	}
}

// =========== GetSecret ===========
// This function is will get value from a file (a secret in k8s env)

func GetSecret(secret string) (value string) {

	file, err := os.Open(secret)
	CheckErr("Unable to open secret"+secret, err)

	defer file.Close()

	// No error, we can read file
	data, err := ioutil.ReadAll(file)
	CheckErr("Unable to get secret content", err)

	return strings.TrimSuffix(string(data), "\n")
}

// =========== GetPCInformation ===========
// This function get configuration form secrets, and populate struct

func GetPCInformation() *ntnx_api_call.Ntnx_endpoint {

	ReturnValue := new(ntnx_api_call.Ntnx_endpoint)
	var err error

	// Check if CSI Secret is present
	_, err = os.Stat("ntnx-secret/endpoint")

	if err == nil {
		log.Fatalln(symbols["FAIL"], "This script currently does not support CSI conf usage. Stay Tuned")

		/*
			=== To be updated ===

			// If Ok, we get others info
			log.Println(symbols["NEUTRAL"], "  Configuration done with Nutanix CSI Configuration")
			ReturnValue.PE = strings.Replace(value, ":9440", "", -1)
			ReturnValue.Mode = "cert"

			// We can not get information about certificates
			res, value = GetSecret("ntnx-secret/cert")

			type TmpStruct struct {
				Key   string `json:"key"`
				Cert  string `json:"cert"`
				Chain string `json:"chain"`
			}

			var data TmpStruct

			if res {

				err := json.Unmarshal([]byte(value), &data)
				if err != nil {
					log.Fatal(symbols["FAIL"], "Unable to retrieve certs info for PC connection (", err, ")")
				}

				ReturnValue.Cert = data.Cert
				ReturnValue.Chain = data.Chain
				ReturnValue.Key = data.Key

				// Do an API call to get PC name

				// Create return struct for API data
				type TmpStruct struct {
					ClusterUUID    string `json:"clusterUuid"`
					ClusterDetails struct {
						ClusterName string   `json:"clusterName"`
						IPAddresses []string `json:"ipAddresses"`
					}
				}

				var retour []TmpStruct

				// Do API Call
				res = ReturnValue.CallAPIJSON("PE", "get", "/PrismGateway/services/rest/v1/multicluster/cluster_external_state", "", &retour)

				if !res {
					log.Fatalln("FAIL", " Unable to get PC information")
				}

				// Store PC value
				ReturnValue.PC = retour[0].ClusterDetails.IPAddresses[0]

			} else {
				log.Fatal(symbols["FAIL"], "Unable to retrieve certs info for PC connection")
			}

		*/

	} else {
		// If not, we get info from dedicated secret

		log.Println(symbols["NEUTRAL"], "  Configuration done with dedicated secret")

		ReturnValue.PC = GetSecret("secret/endpoint")
		ReturnValue.Mode = "password"
		ReturnValue.User = GetSecret("secret/username")
		ReturnValue.Password = GetSecret("secret/password")

	}

	return ReturnValue
}

// =========== ActivateSSLCheck ===========
func ActivateSSLCheck(value bool) string {
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: !value}
	if value {
		return colors["GREEN"] + "activated" + colors["END"]
	} else {
		return colors["RED"] + "deactivated" + colors["END"]
	}
}

// =========== GetKubeconfig ===========
// This function is will get kubeconfig from NKE Cluster and create local file to store it

func GetKubeconfig(endpoint *ntnx_api_call.Ntnx_endpoint, cluster string, target string) {

	type StructTmp struct {
		KubeConfig string `json:"kube_config"`
	}

	var returnValue StructTmp

	url := "/karbon/v1/k8s/clusters/" + cluster + "/kubeconfig"

	endpoint.CallAPIJSON("PC", "GET", url, "", &returnValue)

	f, err := os.Create(target)
	CheckErr("Unable to create kubeconfig file", err)

	defer f.Close()
	f.Write([]byte(returnValue.KubeConfig))
}

// =========== k8s_request ===========
// This function is will do request on k8s cluster

func k8s_request(kubeconfig string, request string, options string) [3]float64 {

	var ReturnValue [3]float64
	var UsedResources = map[string]float64{"cpu": 0, "ram": 0, "pods": 0}
	var AvailableResources = map[string]float64{"cpu": 0, "ram": 0, "pods": 0}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	CheckErr("Load kubeconfig", err)

	clt_metrics, err := metricsv.NewForConfig(config)
	CheckErr("Create connector on k8s cluster", err)

	clt_core, err := kubernetes.NewForConfig(config)
	CheckErr("Create connector on k8s cluster", err)

	switch request {
	case "podnumbers":
		answer, err := clt_core.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{})
		CheckErr("K8S request failed", err)
		ReturnValue[0] = float64(len(answer.Items))
	case "nodenumbers":
		answer, err := clt_core.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{LabelSelector: "kubernetes.io/role=node"})
		CheckErr("Get worker list failed", err)
		ReturnValue[0] = float64(len(answer.Items))
	case "testmetrics":
		_, err := clt_metrics.MetricsV1beta1().PodMetricses("").List(context.TODO(), metav1.ListOptions{})
		CheckErr("K8s Metrics test", err)
		ReturnValue[0] = 0
	case "load":

		// Get usage metrics
		answer, err := clt_metrics.MetricsV1beta1().NodeMetricses().List(context.TODO(), metav1.ListOptions{})
		CheckErr("K8s Metrics collect", err)

		for _, valuestmp := range answer.Items {

			if valuestmp.ObjectMeta.Labels["kubernetes.io/role"] == "node" {

				// Put values in the map
				UsedResources["cpu"] += float64(valuestmp.Usage.Cpu().MilliValue())
				UsedResources["ram"] += float64(valuestmp.Usage.Memory().MilliValue())

				// We need another request for pods
				// Does not return other value than 0 => valuestmp.Usage.Pods()
				answer3, err := clt_core.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{FieldSelector: "spec.nodeName=" + valuestmp.Name})
				CheckErr("K8s pod Metrics", err)

				UsedResources["pods"] += float64(len(answer3.Items))
			}
		}

		// Get available resources per nodes
		answer2, err := clt_core.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
		CheckErr("K8s get available resources", err)

		for _, valuestmp2 := range answer2.Items {

			if valuestmp2.ObjectMeta.Labels["kubernetes.io/role"] == "node" {
				// Put values in the map
				AvailableResources["cpu"] += float64(valuestmp2.Status.Allocatable.Cpu().MilliValue())
				AvailableResources["ram"] += float64(valuestmp2.Status.Allocatable.Memory().MilliValue())
				AvailableResources["pods"] += float64(valuestmp2.Status.Allocatable.Pods().Value())
			}
		}

		// Do the ratio calculation
		ReturnValue[0] = UsedResources["cpu"] / AvailableResources["cpu"] * 100
		ReturnValue[1] = UsedResources["ram"] / AvailableResources["ram"] * 100
		ReturnValue[2] = UsedResources["pods"] / AvailableResources["pods"] * 100
	}

	return ReturnValue
}

// =========== DisplayClusterLoad ===========
// This function display load with color codes

func AnalyseClusterLoad(Load [3]float64, cfg *ini.File) int {
	var dispcolor, line string
	var scaling = map[string]int{"CPU": 0, "RAM": 0, "PODS": 0}

	cfg.Section("Main").Key("cpu_high_limit").MustInt()

	// Display
	line = fmt.Sprintf("Load of cluster '" + cfg.Section("Main").Key("nke_cluster").MustString("") + "' - ")

	// Check cpu
	if Load[0] > cfg.Section("Main").Key("cpu_high_limit").MustFloat64() {
		dispcolor = "RED"
		scaling["CPU"] = 1
	} else if Load[0] < cfg.Section("Main").Key("cpu_low_limit").MustFloat64() {
		dispcolor = "RED"
		scaling["CPU"] = -1
	} else {
		dispcolor = "GREEN"
	}

	line += fmt.Sprintf("CPU : %s%.2f%%%s, ", colors[dispcolor], Load[0], colors["END"])

	// Check memory
	if Load[1] > cfg.Section("Main").Key("ram_high_limit").MustFloat64() {
		dispcolor = "RED"
		scaling["RAM"] = 1
	} else if Load[1] < cfg.Section("Main").Key("ram_low_limit").MustFloat64() {
		dispcolor = "RED"
		scaling["RAM"] = -1
	} else {
		dispcolor = "GREEN"
	}

	line += fmt.Sprintf("RAM : %s%.2f%%%s, ", colors[dispcolor], Load[1], colors["END"])

	// Check cpu
	if Load[2] > cfg.Section("Main").Key("pods_high_limit").MustFloat64() {
		dispcolor = "RED"
		scaling["PODS"] = 1
	} else if Load[2] < cfg.Section("Main").Key("pods_low_limit").MustFloat64() {
		dispcolor = "RED"
		scaling["PODS"] = -1
	} else {
		dispcolor = "GREEN"
	}

	line += fmt.Sprintf("Pods : %s%.2f%%%s", colors[dispcolor], Load[2], colors["END"])
	log.Println(symbols["NEUTRAL"]+"  ", line)

	// Return scaling recommendation
	if scaling["CPU"] == 1 || scaling["RAM"] == 1 || scaling["PODS"] == 1 {
		return 1
	} else if scaling["CPU"] == -1 || scaling["RAM"] == -1 || scaling["PODS"] == -1 {
		return -1
	} else {
		return 0
	}
}

// =========== ScaleClusterTo ===========
// Call NKE API to change worker numbers
func ScaleClusterTo(PC *ntnx_api_call.Ntnx_endpoint, cluster string, number int, nodepool string, timetowait int) {
	var url string

	// Check cluster status
	status, messages := CheckNKEClusterStatus(PC, cluster)
	if !status {
		log.Println(symbols["NOTICE"], "  Cluster scaling skipped, cluster is not Healthy (", messages, ")")
		return
	}

	// Handle NodePool
	if nodepool == "" {
		// No nodepool provided, we have to define it
		url = "/karbon/v1-alpha.1/k8s/clusters/" + cluster + "/node-pools"

		type TmpStruct []struct {
			Category     string `json:"category"`
			Default      bool   `json:"default"`
			Name         string `json:"name"`
			NumInstances int    `json:"num_instances"`
		}

		var ReturnValue TmpStruct

		// Get list of node pools
		PC.CallAPIJSON("PC", "GET", url, "", &ReturnValue)

		for _, value := range ReturnValue {
			if value.Category == "worker" && value.Default == true {
				nodepool = value.Name
				log.Println(symbols["NEUTRAL"], "    Workers node pool found :", nodepool)
			}
		}

	} else {
		fmt.Println(symbols["NEUTRAL"], "    Workers node pool used :", nodepool)
	}

	// Now we can do the call to scale clsuter
	payload := `{
		"count": 1
		}`

	// Define URL regarding number (negative / positive)
	if number == 1 {
		url = "/karbon/v1-alpha.1/k8s/clusters/" + cluster + "/node-pools/" + nodepool + "/add-nodes"
	} else {
		url = "/karbon/v1-alpha.1/k8s/clusters/" + cluster + "/node-pools/" + nodepool + "/remove-nodes"
	}

	type TmpStruct struct {
		TaskUUID string `json:"task_uuid"`
	}

	var ReturnValue TmpStruct

	PC.CallAPIJSON("PC", "POST", url, payload, &ReturnValue)

	log.Println(symbols["NEUTRAL"], "    Scaling started.")
	log.Println(symbols["WAIT"], "    Waiting for end of task", ReturnValue.TaskUUID)

	// We wait for Task
	status, message, message_detail := PC.WaitForTask(ReturnValue.TaskUUID)
	if !status {
		log.Fatalln(symbols["FATAL"], "    Task failed", message, message_detail)
	} else {
		log.Println(symbols["OK"], "    Task succesfull. Scaling OK")
	}

	// We wait for metrics update
	if number == 1 {
		log.Println(symbols["WAIT"], "    We wait", timetowait, "s for metric updates")
		time.Sleep(time.Duration(timetowait) * time.Second)
	}

}

// =========== CheckNKEClusterStatus ===========
// Check status of NKE Cluster
func CheckNKEClusterStatus(PC *ntnx_api_call.Ntnx_endpoint, cluster string) (bool, []string) {
	url := "/karbon/v1/k8s/clusters/" + cluster + "/health"

	type TmpStruct struct {
		Messages []string `json:"messages"`
		Status   bool     `json:"status"`
	}

	var ReturnValue TmpStruct

	PC.CallAPIJSON("PC", "GET", url, "", &ReturnValue)

	return ReturnValue.Status, ReturnValue.Messages

}
