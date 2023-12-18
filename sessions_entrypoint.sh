#!/bin/bash

# Update resolve.conf
source /app/updateresolvconf.sh

JUPYTER_TOKEN=$(uuidgen | tr 'A-Z' 'a-z')
OfficePy__Jupyter__Token=$JUPYTER_TOKEN
export JUPYTER_TOKEN
echo "JUPYTER_TOKEN: $JUPYTER_TOKEN"

OfficePy__ComputeResourceKey=$(uuidgen | tr 'A-Z' 'a-z')
OfficePy__VmKey=$OfficePy__ComputeResourceKey
export OfficePy__ComputeResourceKey
echo "OfficePy__ComputeResourceKey: $OfficePy__ComputeResourceKey"


# Run conda jupyter under user 'jovyan'
runuser jovyan -c 'export OfficePy__ComputeResourceKey=acasessions-redacted; export OfficePy__VmKey=acasessions-redacted; export SYS_RUNTIME_PLATFORM="AzureContainerApps-Sessions"; export AZURE_CODE_EXEC_ENV="AzureContainerApps-Sessions-OfficeImage"; export AZURECONTAINERAPPS_SESSIONS_PLATFORM_VERSION="775818"; conda run --no-capture-output -p /app/officepy /app/condaentrypoint.sh' &

# Launch an HTTP Proxy that will immediately fail the request instead of timeout after 90 seconds.
dotnet /app/httpproxy/httpproxy.dll http://*:8000/ &

# Start acamanager
OfficePy__ComputeResourceKey=$OfficePy__ComputeResourceKey OfficePy__VmKey=$OfficePy__ComputeResourceKey JUPYTER_TOKEN=$JUPYTER_TOKEN OfficePy__Jupyter__Token=$JUPYTER_TOKEN /app/acamanager &
echo "ProxyApp Started, exit code: $?"

# Launch code exec service
OfficePy__ComputeResourceKey=$OfficePy__ComputeResourceKey OfficePy__VmKey=$OfficePy__ComputeResourceKey JUPYTER_TOKEN=$JUPYTER_TOKEN OfficePy__Jupyter__Token=$JUPYTER_TOKEN dotnet /app/service/Microsoft.OfficePy.Service.CodeExecutionService.dll
