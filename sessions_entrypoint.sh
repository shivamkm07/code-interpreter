#!/bin/bash

# Update resolve.conf
source /app/updateresolvconf.sh

# Run conda jupyter under user 'jovyan'
runuser jovyan -c 'conda run --no-capture-output -p /app/officepy /app/condaentrypoint.sh' &

# Launch an HTTP Proxy that will immediately fail the request instead of timeout after 90 seconds.
dotnet /app/httpproxy/httpproxy.dll http://*:8000/ &

# Start proxyapp
/app/proxyapp &> /app/proxyapp.log &
echo "ProxyApp Started, exit code: $?"

# Launch service
dotnet /app/service/Microsoft.OfficePy.Service.CodeExecutionService.dll
