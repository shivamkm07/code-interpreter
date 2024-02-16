# Sessions Code Interpreter

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
   docker build -t jupyterpython .
   ```
3. Run the image:
   ```bash
   docker run -p 8080:8080 jupyterpython
   ```
After running these steps, the the interpreter server should be accessible at `http://localhost:8080`.

### Using the APIs
1. Execute Code - Pass Conditions:
   ```bash
   curl -v -X 'POST' 'http://localhost:8080/execute'   -H 'Content-Type: application/json' -d '{ "code": "1+1" }'

    curl -v -X 'POST' 'http://localhost:8080/execute'   -H 'Content-Type: application/json' -d '{ "code": "import time \ntime.sleep(5) \nprint(\"Done Sleeping\")" }'

    curl -v -X 'POST' 'http://localhost:8080/execute'   -H 'Content-Type: application/json' -d '{ "code": "print(\"Hello Earth\")" }'

    curl -v -X 'POST' 'http://localhost:8080/execute'   -H 'Content-Type: application/json'   -d '{"code": "import matplotlib.pyplot as plt \nimport numpy as np \nx = np.linspace(-2*np.pi, 2*np.pi, 1000) \ny = np.tan(x) \nplt.plot(x, y) \nplt.ylim(-10, 10) \nplt.title('\''Tangent Curve'\'') \nplt.xlabel('\''x'\'') \nplt.ylabel('\''tan(x)'\'') \nplt.grid(True) \nplt.show()"}'
   ```

2. Execute Code - Pass Conditions:
   ```bash
    curl -v -X 'POST' 'http://localhost:8080/execute'   -H 'Content-Type: application/json' -d '{ "code": "printf(\"Hello Earth\")" }'
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