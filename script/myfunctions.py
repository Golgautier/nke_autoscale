#import kubernetes
import requests
import json
from requests.auth import HTTPBasicAuth
import statistics
from kubernetes import client, config
#from kubernetes.client.rest import ApiException
from kubernetes.client.api_client import ApiClient
from jsonpath_ng.ext import parse
import time


# ==================== Class for colors ====================

class bcolors:
    HEADER = '\033[95m'
    OKBLUE = '\033[94m'
    OKCYAN = '\033[96m'
    OKGREEN = '\033[92m'
    WARNING = '\033[93m'
    FAIL = '\033[91m'
    ENDC = '\033[0m'
    BOLD = '\033[1m'
    UNDERLINE = '\033[4m'

# ==================== ConvertValue ====================
def ConvertValue( val ):
    # This fonction convert value found with k8s client like : 12344Mi or 23214Ki

    # Memory, target value is Gi
    if( "Ki" in val ):
        return ( int(val.replace("Ki","")) / (1024*1024) )
    if( "Mi" in val ):
        return ( int(val.replace("Mi","")) / 1024 )

    # CPU, target is m
    if( "m" in val):
        return ( int(val.replace("m","")))

    print(bcolors.FAIL+"ERROR in conversion of load")


# ==================== GetKubeconfig ====================
def GetKubeconfig( pc_ip, pc_user, pc_pass, nke_cluster , check_ssl = False):

    # Create API call request
    url = "https://"+pc_ip+":9440/karbon/v1/k8s/clusters/"+nke_cluster+"/kubeconfig"
    headers = {'Content-Type': 'application/json', 'Accept': 'application/json'}
    
    # Execute the request
    resp = requests.get(url, headers=headers,  auth = HTTPBasicAuth(pc_user, pc_pass), verify=check_ssl)

    if resp.ok:
        resp_json=json.loads(resp.content)

        file = open( "kubeconfig.yaml", "w")
        file.write(resp_json['kube_config'])
        file.close()

        print(bcolors.OKCYAN+"  . Ok"+bcolors.ENDC)
    else:
        print(bcolors.FAIL+"  . Unable to get kubeconfig file ( Error :",resp.status_code,")")
        print("  . "+resp.text+bcolors.ENDC)
        exit(2)

# ==================== GetWorkerStats ====================
def GetWorkerStats( kubeconfig ):
    
    stats_nodes = { 'cpu': {}, 'ram': {}, 'pod': {}}

    # Load Kubeconfig
    try:
        config.load_kube_config( kubeconfig )
    except:
        print(bcolors.FAIL+"  . Impossible to find proper kubeconfig"+bcolors.ENDC)
        return 0,0,0,0,"kubeconfig"
    else:
        v1 = client.CoreV1Api()
        api_client = ApiClient()

        # Check connection
        try:
            v1.list_namespace(watch=False)
        except:
            print(bcolors.FAIL+"  . Impossible to connect on k8s cluster"+bcolors.ENDC)
            return 0,0,0,0,"kubeconfig"
        
        # We check all nodes to get only ready nodes, and collect capacity for CPU, RAM & pods
        for n in v1.list_node().items:
            role = n.metadata.labels["kubernetes.io/role"]
            for status in n.status.conditions:
                if ( status.status == 'True' ) and ( status.type == 'Ready' ) and ( role == "node" ) :
                    
                    # We get usage for this cluster
                    response = api_client.call_api( "/apis/metrics.k8s.io/v1beta1/nodes/"+n.metadata.name , 'GET', auth_settings=['BearerToken'], response_type='json', _preload_content=False)
                    response_json = json.loads(response[0].data.decode('utf-8'))

                    # Specific request to get pod nombers on the node
                    field_selector = 'spec.nodeName='+n.metadata.name
                    response2 = v1.list_pod_for_all_namespaces(watch=False, field_selector=field_selector)
                    node_pods_number=len(response2.items)
                    
                    stats_nodes['cpu'][n.metadata.name] = ConvertValue(response_json['usage']['cpu']) / (int(n.status.capacity['cpu']) * 1000) * 100
                    stats_nodes['ram'][n.metadata.name] = ConvertValue(response_json['usage']['memory']) / ConvertValue(n.status.capacity['memory']) * 100
                    stats_nodes['pod'][n.metadata.name] = node_pods_number / int(n.status.capacity['pods']) * 100

        return len(stats_nodes['cpu']),statistics.mean(list(stats_nodes['cpu'].values())),statistics.mean(list(stats_nodes['ram'].values())),statistics.mean(list(stats_nodes['pod'].values())),"ok"
    
# ==================== ScaleNKECluster ====================
def ScaleNKECluster( cluster_name, pc_ip, pc_user, pc_pass, node_number_change , node_pool, check_ssl=False ):

    if ( node_pool == "" ):
        headers = {'Content-Type': 'application/json', 'Accept': 'application/json'}
        url = "https://"+pc_ip+":9440/karbon/v1-alpha.1/k8s/clusters/"+cluster_name+"/node-pools"

        resp = requests.get(url, headers=headers, auth = HTTPBasicAuth(pc_user, pc_pass), verify=check_ssl)
        resp_json = json.loads(resp.content)

        # Get worker pool with more than 1 node
        jsonpath_expression = parse("$[?((@.num_instances >= 1) & (@.category=='worker')) ].name")
        
        node_pool = jsonpath_expression.find( resp_json )[0].value
        print(bcolors.WARNING+"  . Node pool automatically selected : "+node_pool+bcolors.ENDC)
        
    # Create the Karbon Kubernetes cluster
    # Set the headers, payload, and cookies
    headers = {'Content-Type': 'application/json', 'Accept': 'application/json'}
    payload = {
        "count": 1
    }
    
    # Set the address and make API call
    if ( node_number_change == 1 ):
        url = "https://"+pc_ip+":9440/karbon/v1-alpha.1/k8s/clusters/"+cluster_name+"/node-pools/"+node_pool+"/add-nodes"
    else:
        url = "https://"+pc_ip+":9440/karbon/v1-alpha.1/k8s/clusters/"+cluster_name+"/node-pools/"+node_pool+"/remove-nodes"

    resp = requests.post(url, data=json.dumps(payload), headers=headers, auth = HTTPBasicAuth(pc_user, pc_pass), verify=check_ssl)

    if resp.ok:
        answer = json.loads(resp.content)
        return( answer['task_uuid'])
    else:
        return( "error" )

# ==================== CheckTask ====================
def CheckTask( task_uuid, pc_ip, pc_user, pc_pass, check_ssl):
    
        if ( task_uuid == "error" ):
            return False
        
        headers = {'Content-Type': 'application/json', 'Accept': 'application/json'}
        url = "https://"+pc_ip+":9440/api/nutanix/v3/tasks/"+task_uuid

        completion=0

        # Wait for completion    
        while ( completion < 100 ):

            time.sleep(10)

            resp = requests.get(url, headers=headers, auth = HTTPBasicAuth(pc_user, pc_pass), verify=check_ssl)
            resp_json = json.loads(resp.content)

            completion=int(resp_json['percentage_complete'])

        # Do return
        if ( resp_json['status'] == "SUCCEEDED"):
            return True
        else:
            return False

# ==================== CheckNKEClusterStatus ====================
def CheckNKEClusterStatus( cluster_name, pc_ip, pc_user, pc_pass, check_ssl=False):
    
    headers = {'Content-Type': 'application/json', 'Accept': 'application/json'}
    url = "https://"+pc_ip+":9440/karbon/v1/k8s/clusters/"+cluster_name+"/health"

    resp = requests.get(url, headers=headers, auth = HTTPBasicAuth(pc_user, pc_pass), verify=check_ssl)
    resp_json = json.loads(resp.content)

    return(bool(resp_json['status']))

# ==================== CheckMetrics ====================
def CheckMetrics( kubeconfig ):
    # Load Kubeconfig
    try:
        config.load_kube_config( kubeconfig )
    except:
        print(bcolors.FAIL+"  . Impossible to find proper kubeconfig"+bcolors.ENDC)
        exit(4)
    else:
        try:
            api_client = ApiClient()
            api_client.call_api( "/apis/metrics.k8s.io/v1beta1/nodes" , 'GET', auth_settings=['BearerToken'], response_type='json', _preload_content=False)
        except :
            print(bcolors.FAIL+"  . Error. Please verify metrics on the k8s cluster"+bcolors.ENDC)
            exit(6)
        else:
            print(bcolors.OKCYAN+"  . Ok"+bcolors.ENDC)
    
