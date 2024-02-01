#!/bin/bash

JUPYTER_GEN_TOKEN=$(uuidgen | tr 'A-Z' 'a-z')
OfficePy__Jupyter__Token=$JUPYTER_GEN_TOKEN
export JUPYTER_GEN_TOKEN
echo "JUPYTER_GEN_TOKEN: $JUPYTER_GEN_TOKEN"

# Start the Go app in the background
./goclientapp &

# Start Jupyter notebook
JUPYTER_TOKEN="test" jupyter notebook --ip=0.0.0.0 --port=8888 --no-browser --allow_origin='*' --allow-root