# README.md

## Steps to Run the Project

Follow these steps to set up and run the e2e tests in local:

### Prerequisites

Before you begin, ensure you have the following installed:

- Docker: [Install Docker](https://docs.docker.com/get-docker/)

### Installing

To install and run the e2e test, follow these steps:

1. Clone the repository to your local machine:
   ```bash
   git clone https://github.com/your-username/jupyterpython.git
   cd jupyterpython
   ```

2. **Build Docker image**
   - This step builds a Docker image for the project.
   - Command: `make build-jupyterpython-image`

3. **Run Docker container**
   - This step runs a Docker container for the project.
   - Command: `make run-jupyterpython-container`

4. **Run tests**
   - This step runs end-to-end tests for the project.
   - Command: `make test-e2e-all`

5. **Delete Docker container**
   - This step deletes the Docker container used for the project.
   - Command: `make delete-jupyterpython-container-windows`
