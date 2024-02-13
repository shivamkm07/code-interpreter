#!/bin/bash

JUPYTER_GEN_TOKEN=$(uuidgen | tr 'A-Z' 'a-z')
export JUPYTER_GEN_TOKEN
echo "JUPYTER_GEN_TOKEN: $JUPYTER_GEN_TOKEN"

# Verify the installation
conda --version

# Start Jupyter notebook
echo "Starting Jupyter.."
conda run -p /app/condaapp env JUPYTER_TOKEN=$JUPYTER_GEN_TOKEN jupyter notebook --ip=0.0.0.0 --port=8888 --no-browser --allow-root &

# sleep for 10 seconds to allow Jupyter notebook to start
sleep 10

# Start the Go app
echo "Starting Go app.."
./goclientapp