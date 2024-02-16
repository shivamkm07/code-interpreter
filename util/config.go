package util

import (
	"context"

	"github.com/sethvargo/go-envconfig"
)

type JupyterPythonConfig struct {
	UseTls             string `env:"USE_TLS,default=false"`
	XdsCertFilePath    string `env:"XDS_CERT_FILE_PATH,default=/etc/jupyterpython/certs/cert.pem"`
	XdsCertKeyFilePath string `env:"XDS_CERT_KEY_FILE_PATH,default=/etc/jupyterpython/certs/key.pem"`
}

var values = JupyterPythonConfig{}

func init() {
	err := envconfig.Process(context.TODO(), &values)
	if err != nil {
		panic(err)
	}
}

func GetConfig() JupyterPythonConfig {
	return values
}
