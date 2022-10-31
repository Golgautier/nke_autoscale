# Author : Gautier LEBLANC
# Purpose : Manage autoscale capability for Nutanix Kubernetes Engine.
#           This script will be executed in a container

import configparser
#from gettext import ngettext
import myfunctions
from myfunctions import bcolors as b
import time
from datetime import datetime
#from kubernetes import client, config

# Global variables
secret_dir = "secret"
alerts = { 'cpu': 0, 'ram': 0, 'pods': 0}

# Get PC credentials
file = open( secret_dir+"/endpoint", 'r')
pc_ip=file.readlines()[0].rstrip()
file.close()

file = open( secret_dir+"/username")
pc_user = file.readlines()[0].rstrip()
file.close()

file = open( secret_dir+"/password")
pc_pass = file.readlines()[0].rstrip()
file.close()

# Get config info
myconfig=configparser.ConfigParser()
myconfig.read("config/config.ini")

# To start, we need to get KubeConfig
print(b.OKBLUE+"Getting Kubeconfig file to start..."+b.ENDC)
myfunctions.GetKubeconfig( pc_ip, pc_user, pc_pass, myconfig['Main']['nke_cluster'], bool(myconfig['Main']['check_ssl']) )

# Check metrics
print(b.OKBLUE+"Checking metrics availability..."+b.ENDC)
myfunctions.CheckMetrics( "kubeconfig.yaml" )

print(b.OKBLUE+"\nWe start monitoring cluster "+myconfig['Main']['nke_cluster']+". Pool frequency : "+myconfig['Main']['poolfrequency']+"s"+b.ENDC)

# We start looping
while True:
    # We look at cluster nodes status
    node_number, avg_cpu, avg_ram, avg_pods, comment = myfunctions.GetWorkerStats( "kubeconfig.yaml" )
    
    # Get Time for timestamp
    now=datetime.now()

    if ( comment == "ok" ):
        if ( avg_cpu < int(myconfig['Main']['cpu_low_limit'])) or ( avg_cpu > int(myconfig['Main']['cpu_high_limit'])) :
            display_cpu_color = b.FAIL
            alerts['cpu'] += 1
        else:
            display_cpu_color = b.OKGREEN
            alerts['cpu'] = 0
        
        if ( avg_ram < int(myconfig['Main']['ram_low_limit'])) or ( avg_ram > int(myconfig['Main']['ram_high_limit'])) :
            display_ram_color = b.FAIL
            alerts['ram'] += 1
        else:
            display_ram_color = b.OKGREEN
            alerts['ram'] = 0

        if ( avg_pods < int(myconfig['Main']['pods_low_limit'])) or ( avg_cpu > int(myconfig['Main']['pods_high_limit'])) :
            display_pods_color = b.FAIL
            alerts['pods'] += 1
        else:
            display_pods_color = b.OKGREEN
            alerts['pods'] = 0
        
        # Display result
        print(now.strftime("%m/%d/%Y %H:%M:%S")+": Average CPU used :"+display_cpu_color , round(avg_cpu,2) , "%"+b.ENDC+", Average ram used :"+display_ram_color, round(avg_ram,2) , "%"+b.ENDC+", Average pods used :"+display_pods_color, round(avg_pods,2), "%"+b.ENDC  )

        # Calculate if scaling is needed
        scaling={}
        scale=0

        if( alerts['cpu'] >= int(myconfig['Main']['occurences'])):
            if( avg_cpu > int(myconfig['Main']['cpu_high_limit']) ):
                scaling['cpu']="up"
            else:
                scaling['cpu']="down"

        if( alerts['ram'] >= int(myconfig['Main']['occurences'])):
            if( avg_ram > int(myconfig['Main']['ram_high_limit']) ):
                scaling['ram']="up"
            else:
                scaling['ram']="down"

        if( alerts['pods'] >= int(myconfig['Main']['occurences'])):
            if( avg_pods > int(myconfig['Main']['pods_high_limit']) ):
                scaling['pods']="n/a"
            else:
                scaling['pods']="n/a"

        # If one of the 3 axis require scale-out, ask for it...
        if ( "up" in scaling.values()) and ( node_number < int(myconfig['Main']['max_nodenumber'])):
            print( b.WARNING+"  . Scale up requested, from",node_number, "to", node_number+1,"worker(s) "+b.ENDC)
            scale=+1
        # If no scale-out is required, and scale-in is...
        elif ( "down" in scaling.values()) and ( node_number > int(myconfig['Main']['min_nodenumber'] )):
            print( b.WARNING+"  . Scale in requested, from",node_number, "to", node_number-1,"worker(s) "+b.ENDC)
            scale=-1

        if( scale != 0 ):
            if ( myfunctions.CheckNKEClusterStatus( myconfig['Main']['nke_cluster'], pc_ip, pc_user, pc_pass, bool(myconfig['Main']['check_ssl']) ) ):
                task_uuid = myfunctions.ScaleNKECluster( myconfig['Main']['nke_cluster'], pc_ip, pc_user, pc_pass, scale, myconfig['Main']['node_pool'], bool(myconfig['Main']['check_ssl']) )
                print( b.WARNING+"  . Scaling initiated, waiting for end of task ("+task_uuid+")"+b.ENDC) 

                # Waiting of task initialisation
                time.sleep(30)

                if( myfunctions.CheckTask(task_uuid, pc_ip, pc_user, pc_pass, bool(myconfig['Main']['check_ssl']))):
                    print(b.OKGREEN+"  . Finished. Ok"+b.ENDC)
                else:
                    print(b.FAIL+"  . Finished. Failed"+b.ENDC)

                if ( scale == 1):
                    print(b.WARNING+"  . Waiting "+myconfig['Main']['wait_after_scaleout']+" more for metrics update..."+b.ENDC)
                    time.sleep(int(myconfig['Main']['wait_after_scaleout']))
            else:
                print(b.WARNING+"  . Scaling skipped because of cluster not in a ready state"+b.ENDC)
                time.sleep(int(myconfig['Main']['poolfrequency']))
        else:
            # Waiting before restart check
            time.sleep(int(myconfig['Main']['poolfrequency']))

    elif ( comment == "kubeconfig" ):
        print( b.FAIL+"  . Kubeconfig seems to have expired, try to get a new one..."+b.ENDC)
        myfunctions.GetKubeconfig( pc_ip, pc_user, pc_pass, myconfig['Main']['nke_cluster'], bool(myconfig['Main']['check_ssl']) )