#!/bin/bash

if [ $# -ne 1 ]
then
    echo "Usage : $0 <image name>"
    exit 2
fi

cp ../script/autoscale.py ./script/
cp ../script/myfunctions.py ./script/
cp ../script/requirements.txt ./script/

podman build . -t $1