# Jupyter Python

A fork of Office AI Image with a custom handler for managing the Jupyter Python Code Interpreter. This project is designed to provide the backend for ACA Sessions Python Code Interpreter.

## Getting Started

To get started with this project, you'll need to have Docker installed on your machine. The project is containerized, which means you can easily build and run it in a Docker environment.

### Prerequisites

Before you begin, ensure you have the following installed:

- Docker: [Install Docker](https://docs.docker.com/get-docker/)

### Installing

To install and run the project, follow these steps:

1. Clone the repository to your local machine:
   ```bash
   git clone https://github.com/your-username/jupyterpython.git
   cd jupyterpython
   ```
2. Build container image
   ```bash
   docker build -t jupyterpython . -t cappsinttestregistrypublic.azurecr.io/codeexecjupyter:v7758
   ```
3. Run the image:
   ```bash
   docker run -p 6000:6000 -e OfficePy__Jupyter__Token=test -e JUPYTER_TOKEN=test -e OfficePy_LocalhostDeployment=true -e DATA_UPLOAD_PATH="/mnt/data" cappsinttestregistrypublic.azurecr.io/codeexecjupyter:v7758
   ```
After running these steps, the the interpreter server should be accessible at `http://localhost:6000`.

### Using the APIs
1. Listing Files List all files in the `/mnt/data` directory:
   ```bash
   curl http://localhost:6000/listfiles
   ```

2. Upload a file to `/mnt/data`:
   ```bash
   curl -X POST -F "file=@/path/to/your/file.txt" http://localhost:6000/upload
   ```
  
3. Download a file from `/mnt/data`:
   ```bash
   curl -O http://localhost:6000/download/filename.txt
   ```

4. Execute Code:
   ```bash
   curl -H "Authorization: ApiKey /computeresourcekey123" -X 'POST'   'http://localhost:6000/execute'   -H 'Content-Type: application/json' -d '{ "code": "1+1" }'
   ```

5. Validate the Health of the container:
   ```bash
   curl http://localhost:6000/healthz -v
   ```

# Contributing

This project welcomes contributions and suggestions. Most contributions require
you to agree to a Contributor License Agreement (CLA) declaring that you have
the right to, and actually do, grant us the rights to use your contribution.
For details, visit https://cla.opensource.microsoft.com.

When you submit a pull request, a CLA bot will automatically determine whether
you need to provide a CLA and decorate the PR appropriately (e.g., status
check, comment). Simply follow the instructions provided by the bot. You will
only need to do this once across all repos using our CLA.
