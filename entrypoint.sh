#!/bin/bash

# Start the Go app in the background
/app &

# Start Jupyter notebook
jupyter notebook --ip=0.0.0.0 --port=8888 --no-browser --allow_origin='*' --allow-root