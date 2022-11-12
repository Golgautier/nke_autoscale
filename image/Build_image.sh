#!/bin/bash

if [ $# -ne 1 ]
then
    echo "Usage : $0 <image name>"
    exit 2
fi

DIR=`pwd`

# Clean (if previous execution failed)
rm -rf script/ 2>/dev/null
mkdir script 2>/dev/null

# Get 
cp ../script/go.mod script/
cp ../script/*.go script/
cp ../script/go.sum script/
cp -r ../script/ntnx_api_call script/

# Create image
podman build . -t $1

# Clean
rm -rf script/ 2>/dev/null
mkdir script 2>/dev/null