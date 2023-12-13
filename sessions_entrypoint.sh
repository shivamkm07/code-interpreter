#!/bin/bash

# Update resolve.conf
source /app/updateresolvconf.sh

JUPYTER_TOKEN=$(uuidgen | tr 'A-Z' 'a-z')
OfficePy__Jupyter__Token=$JUPYTER_TOKEN

echo "JUPYTER_TOKEN: $JUPYTER_TOKEN"
echo "OfficePy__Jupyter_Token: $OfficePy__Jupyter__Token"

OfficePy__ComputeResourceKey=$(uuidgen | tr 'A-Z' 'a-z')
OfficePy__VmKey=$OfficePy__ComputeResourceKey

# Run conda jupyter under user 'jovyan'
runuser jovyan -c 'export OfficePy__ComputeResourceKey=aca-securetoken;export SYS_RUNTIME_PLATFORM="AzureContainerApps-Sessions";export AZURE_CODE_EXEC_ENV="AzureContainerApps-Sessions-OfficeImage"; export AZURECONTAINERAPPS_SESSIONS_PLATFORM_VERSION="775818";conda run --no-capture-output -p /app/officepy /app/condaentrypoint.sh' &


# Launch an HTTP Proxy that will immediately fail the request instead of timeout after 90 seconds.
dotnet /app/httpproxy/httpproxy.dll http://*:8000/ &

# Start acamanager
/app/acamanager &
echo "ProxyApp Started, exit code: $?"

# Launch code exec service
OfficePy__Jupyter__Token=$JUPYTER_TOKEN dotnet /app/service/Microsoft.OfficePy.Service.CodeExecutionService.dll
