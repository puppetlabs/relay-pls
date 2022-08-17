package opt

import (
	"net/url"

	"github.com/spf13/viper"
)

const (
	DefaultMetricsURL       = "http://localhost:3050"
	DefaultVaultEngineMount = "pls"
	DefaultVaultURL         = "http://localhost:8200"
)

type Config struct {
	Debug bool

	MetricsEnabled bool
	MetricsAddr    string

	ListenPort int

	Dataset string
	Project string
	Table   string

	VaultAddr                 *url.URL
	VaultToken                string
	VaultEngineMount          string
	OAuthVaultEngineMountRoot string
}

func NewConfig() (*Config, error) {
	viper.SetEnvPrefix("relay_pls")
	viper.AutomaticEnv()

	viper.SetDefault("metrics_enabled", false)
	viper.SetDefault("metrics_server_addr", DefaultMetricsURL)
	viper.SetDefault("vault_engine_mount", DefaultVaultEngineMount)

	config := &Config{
		Debug: viper.GetBool("debug"),

		MetricsEnabled: viper.GetBool("metrics_enabled"),
		MetricsAddr:    viper.GetString("metrics_server_addr"),

		ListenPort: viper.GetInt("listen_port"),

		Dataset: viper.GetString("dataset"),
		Project: viper.GetString("project"),
		Table:   viper.GetString("table"),

		VaultEngineMount: viper.GetString("vault_engine_mount"),
	}

	if viper.IsSet("vault_addr") {
		vaultURL, err := url.Parse(viper.GetString("vault_addr"))
		if err != nil {
			return nil, err
		}

		config.VaultAddr = vaultURL
	}

	return config, nil
}
