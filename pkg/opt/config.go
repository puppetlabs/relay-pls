package opt

import (
	"context"
	"net/http"
	"net/url"

	"cloud.google.com/go/bigquery"
	vaultapi "github.com/hashicorp/vault/api"
	"github.com/spf13/viper"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"google.golang.org/api/googleapi"
)

const (
	DefaultOAuthVaultEngineMountRoot = "oauth"
	DefaultVaultEngineMount          = "pls"
	DefaultVaultURL                  = "http://localhost:8200"
)

type Config struct {
	Debug bool

	ListenPort int

	Dataset string
	Project string
	Table   string

	VaultAddr                 *url.URL
	VaultToken                string
	VaultEngineMount          string
	OAuthVaultEngineMountRoot string
}

func (c *Config) BigQueryTable() (*bigquery.Table, error) {
	client, err := c.BigQueryClient()
	if err != nil {
		return nil, err
	}

	schema := bigquery.Schema{
		{Name: "log_id", Type: bigquery.StringFieldType, Required: true},
		{Name: "log_message_id", Type: bigquery.StringFieldType, Required: true},
		{Name: "timestamp", Type: bigquery.TimestampFieldType, Required: true},
		{Name: "encrypted_payload", Type: bigquery.BytesFieldType},
	}

	metadata := &bigquery.TableMetadata{
		Schema: schema,
		Clustering: &bigquery.Clustering{
			Fields: []string{
				"log_id",
			},
		},
	}

	ctx := context.Background()

	dataset := client.Dataset(c.Dataset)
	table := dataset.Table(c.Table)
	err = table.Create(ctx, metadata)
	if e, ok := err.(*googleapi.Error); ok && e.Code != http.StatusConflict {
		return nil, err
	}

	return table, nil
}

func (c *Config) BigQueryClient() (*bigquery.Client, error) {
	ctx := context.Background()
	return bigquery.NewClient(ctx, c.Project)
}

func (c *Config) VaultClient() (*vaultapi.Client, error) {
	if c.VaultAddr != nil {
		vaultClient, err := vaultapi.NewClient(vaultapi.DefaultConfig())
		if err != nil {
			return nil, err
		}

		if err := vaultClient.SetAddress(c.VaultAddr.String()); err != nil {
			return nil, err
		}

		vaultClient.SetToken(c.VaultToken)

		return vaultClient, nil
	}

	return nil, nil
}

func (c *Config) WithTelemetry() error {
	exporter, err := stdout.NewExporter(stdout.WithPrettyPrint())
	if err != nil {
		return err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithConfig(sdktrace.Config{DefaultSampler: sdktrace.AlwaysSample()}),
		sdktrace.WithSyncer(exporter),
	)
	if err != nil {
		return err
	}

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	return nil
}

func NewConfig() (*Config, error) {
	viper.SetEnvPrefix("relay_pls")
	viper.AutomaticEnv()

	viper.SetDefault("oauth_vault_engine_mount_root", DefaultOAuthVaultEngineMountRoot)
	viper.SetDefault("vault_engine_mount", DefaultVaultEngineMount)

	config := &Config{
		Debug: viper.GetBool("debug"),

		ListenPort: viper.GetInt("listen_port"),

		Dataset: viper.GetString("dataset"),
		Project: viper.GetString("project"),
		Table:   viper.GetString("table"),

		OAuthVaultEngineMountRoot: viper.GetString("oauth_vault_engine_mount_root"),
		VaultEngineMount:          viper.GetString("vault_engine_mount"),
		VaultToken:                viper.GetString("vault_token"),
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
