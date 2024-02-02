#!/bin/bash

JUPYTER_GEN_TOKEN=$(uuidgen | tr 'A-Z' 'a-z')
OfficePy__Jupyter__Token=$JUPYTER_GEN_TOKEN
export JUPYTER_GEN_TOKEN
echo "JUPYTER_GEN_TOKEN: $JUPYTER_GEN_TOKEN"

# Start Jupyter notebook
echo "Starting Jupyter.."
JUPYTER_TOKEN="test" jupyter notebook --ip=0.0.0.0 --port=8888 --no-browser --allow_origin='*' --allow-root &

# sleep for 10 seconds to allow Jupyter notebook to start
sleep 10

# Start the Go app
echo "Starting Go app.."
./goclientapp