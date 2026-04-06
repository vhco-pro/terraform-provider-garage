package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/vhco-pro/terraform-provider-garage/internal/garage"
)

var _ provider.Provider = &GarageProvider{}

type GarageProvider struct {
	version string
}

type GarageProviderModel struct {
	Endpoint     types.String `tfsdk:"endpoint"`
	Token        types.String `tfsdk:"token"`
	Timeout      types.Int64  `tfsdk:"timeout"`
	MaxRetries   types.Int64  `tfsdk:"max_retries"`
	RetryMinWait types.Int64  `tfsdk:"retry_min_wait"`
	RetryMaxWait types.Int64  `tfsdk:"retry_max_wait"`
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &GarageProvider{
			version: version,
		}
	}
}

func (p *GarageProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "garage"
	resp.Version = p.version
}

func (p *GarageProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manage Garage S3-compatible object storage via the Admin API (v2).",
		Attributes: map[string]schema.Attribute{
			"endpoint": schema.StringAttribute{
				Description: "Garage Admin API endpoint URL (e.g., http://localhost:3903). Can also be set with the GARAGE_ENDPOINT environment variable.",
				Optional:    true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						regexpHTTPEndpoint,
						"must be a valid HTTP or HTTPS URL",
					),
				},
			},
			"token": schema.StringAttribute{
				Description: "Garage Admin API bearer token. Can also be set with the GARAGE_TOKEN environment variable.",
				Optional:    true,
				Sensitive:   true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"timeout": schema.Int64Attribute{
				Description: "HTTP request timeout in seconds. Defaults to 30.",
				Optional:    true,
				Validators: []validator.Int64{
					int64validator.Between(1, 300),
				},
			},
			"max_retries": schema.Int64Attribute{
				Description: "Maximum number of retries on transient failures. Defaults to 3.",
				Optional:    true,
				Validators: []validator.Int64{
					int64validator.Between(0, 10),
				},
			},
			"retry_min_wait": schema.Int64Attribute{
				Description: "Minimum wait between retries in seconds. Defaults to 1.",
				Optional:    true,
				Validators: []validator.Int64{
					int64validator.AtLeast(1),
				},
			},
			"retry_max_wait": schema.Int64Attribute{
				Description: "Maximum wait between retries in seconds. Defaults to 30.",
				Optional:    true,
				Validators: []validator.Int64{
					int64validator.AtLeast(1),
				},
			},
		},
	}
}

func (p *GarageProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config GarageProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	endpoint := config.Endpoint.ValueString()
	if config.Endpoint.IsNull() || config.Endpoint.IsUnknown() {
		endpoint = os.Getenv("GARAGE_ENDPOINT")
	}
	if endpoint == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("endpoint"),
			"Missing Garage API Endpoint",
			"The provider cannot create the Garage API client as there is a missing or empty value for the Garage API endpoint. "+
				"Set the endpoint value in the configuration block or use the GARAGE_ENDPOINT environment variable.",
		)
	}

	token := config.Token.ValueString()
	if config.Token.IsNull() || config.Token.IsUnknown() {
		token = os.Getenv("GARAGE_TOKEN")
	}
	if token == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("token"),
			"Missing Garage API Token",
			"The provider cannot create the Garage API client as there is a missing or empty value for the Garage API token. "+
				"Set the token value in the configuration block or use the GARAGE_TOKEN environment variable.",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	var opts []garage.GarageClientOption

	if !config.Timeout.IsNull() && !config.Timeout.IsUnknown() {
		opts = append(opts, garage.WithTimeout(config.Timeout.ValueInt64()))
	}
	if !config.MaxRetries.IsNull() && !config.MaxRetries.IsUnknown() {
		opts = append(opts, garage.WithMaxRetries(int(config.MaxRetries.ValueInt64())))
	}
	if !config.RetryMinWait.IsNull() && !config.RetryMinWait.IsUnknown() && !config.RetryMaxWait.IsNull() && !config.RetryMaxWait.IsUnknown() {
		opts = append(opts, garage.WithRetryWait(config.RetryMinWait.ValueInt64(), config.RetryMaxWait.ValueInt64()))
	}

	client, err := garage.NewGarageClient(endpoint, token, opts...)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Create Garage API Client",
			"An unexpected error occurred when creating the Garage API client: "+err.Error(),
		)
		return
	}

	resp.DataSourceData = client
	resp.ResourceData = client
}

func (p *GarageProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewBucketResource,
		NewBucketAliasResource,
		NewKeyResource,
		NewBucketPermissionResource,
		NewLayoutNodeResource,
		NewAdminTokenResource,
	}
}

func (p *GarageProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewBucketDataSource,
		NewBucketsDataSource,
		NewKeyDataSource,
		NewKeysDataSource,
		NewClusterLayoutDataSource,
		NewClusterHealthDataSource,
		NewClusterStatusDataSource,
		NewAdminTokenDataSource,
		NewAdminTokensDataSource,
		NewNodeInfoDataSource,
	}
}
