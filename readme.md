# Disclaimer

This work is NOT an official Nutanix development. 

# Purpose

This project allows to add auto-scaling feature for Nutanix Kubernetes Engine (NKE, formerly Karbon).

It can be run on the NKE cluster itself, or on another cluster, targeting the (unique) cluster of your choice.

# Principle

nke_autoscale image embed a python script that will monitor a unique NKE cluster.
When resources are insufficient, it launches nodes add-on (one per one) on the cluster, to get a proper situation.
When available resources are too high, it launches cluster scale-in to remove useless nodes (node per node too)
All triggers are customizable (see "usage") through secret and a configmap.

# Usage

## Prepare

Copy deployment/autoscale.yaml on your laptop and modify the different parts :

### ConfigMap

The configmap is used to define all triggers and information relative to the cluster :
    
    poolfrequency       : pause between 2 requests (seconds)
    nke_cluster         : name of the NKE cluster monitored
    cpu_high_limit      : high limit for CPU usage (in percent), will be compared to your average CPU usage 
    cpu_low_limit       : low limit for CPU usage (in percent), will be compared to your average CPU usage 
    ram_high_limit      : high limit for RAM usage (in percent), will be compared to your average RAM usage 
    ram_low_limit       : low limit for RAM usage (in percent), will be compared to your average RAM usage 
    pods_high_limit     : high limit for Pods (in percent), will be compared to your average Pod usage 
    pods_low_limit      : low limit for Pod usage (in percent), will be compared to your average Pod usage 
    occurences          : Number of times you need to reach the triggers before launching the scaling.
    min_nodenumber      : minimal node number authorized in your cluster
    max_nodenumber      : maximal node number authorized in your cluster
    node_pool           : worker pool to use to scale-out or scale-in. Let empty to automated definition by the script.
    wait_after_scaleout : Time (in seconds) after node add-on before requesting new stats
    check_ssl           : Boolean, to define if we bypass SSL verification during API call.


### Secret

Set the 3 secrets values (base64 encoded), for Prism Central Username, Prism Central User Password, and Prism Central FQDN or IP

## Execute

Launch the app with command in the namespace of your choice

    kubectl apply -f autoscale.yaml -n <namespace>

Check if your deployment is runing, with 1 pod running.

## Monitor

Look at the container logs, you should have something like that

    nke-autoscale-67c8c6c994-p7799 Getting Kubeconfig file to start...
    nke-autoscale-67c8c6c994-p7799   . Ok
    nke-autoscale-67c8c6c994-p7799 Checking metrics availability...
    nke-autoscale-67c8c6c994-p7799   . Ok
    nke-autoscale-67c8c6c994-p7799
    nke-autoscale-67c8c6c994-p7799 We start monitoring cluster scale. Pool frequency : 10s
    nke-autoscale-67c8c6c994-p7799 10/31/2022 22:10:53: Average CPU used : 10.19 %, Average ram used : 62.44 %, Average pods used : 28.18 %
    nke-autoscale-67c8c6c994-p7799 10/31/2022 22:11:03: Average CPU used : 10.19 %, Average ram used : 62.44 %, Average pods used : 28.18 %
    nke-autoscale-67c8c6c994-p7799 10/31/2022 22:11:13: Average CPU used : 10.3 %, Average ram used : 61.48 %, Average pods used : 28.18 %
    nke-autoscale-67c8c6c994-p7799 10/31/2022 22:11:23: Average CPU used : 10.3 %, Average ram used : 61.48 %, Average pods used : 28.18 %
    nke-autoscale-67c8c6c994-p7799 10/31/2022 22:11:34: Average CPU used : 9.62 %, Average ram used : 63.99 %, Average pods used : 29.09 %
    nke-autoscale-67c8c6c994-p7799 10/31/2022 22:11:44: Average CPU used : 9.62 %, Average ram used : 63.99 %, Average pods used : 29.09 %
    nke-autoscale-67c8c6c994-p7799 10/31/2022 22:11:54: Average CPU used : 9.62 %, Average ram used : 63.99 %, Average pods used : 29.09 %                          

All scaling operations will obviously displayed in the logs of the pod.

## Update configuration

Do not forget that configmap are not updated in the pod, when you update it. you have to kill the pod to force pod recreation, to get the new configuration.