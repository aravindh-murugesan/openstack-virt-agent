package openstack

import (
	"context"
	"crypto/tls"
	"net/http"
	"os"
	"time"

	"github.com/aravindh-murugesan/openstack-virt-agent/pkg/config"
	"github.com/gophercloud/gophercloud/v2"
	"github.com/gophercloud/gophercloud/v2/openstack"
	"github.com/gophercloud/utils/v2/openstack/clientconfig"
)

type OpenstackClient struct {
	ProfileName        string
	ComputeClient      *gophercloud.ServiceClient
	BlockStorageClient *gophercloud.ServiceClient
	IdentityClient     *gophercloud.ServiceClient

	Region    string
	Interface string

	OperationTimeoutSeconds int
}

// NewOpenstackClient initializes and authenticates a multi-service OpenStack client.
func NewOpenstackClient(openstackConfig config.OpenStackConfig) (*OpenstackClient, error) {
	var c OpenstackClient
	c.OperationTimeoutSeconds = 60 // user requested default

	// Establish a bounded context for the initial authentication phase
	authContext, authcancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer authcancel()

	// Resolve the target cloud environment configuration.
	opts := &clientconfig.ClientOpts{
		Cloud: openstackConfig.CloudName,
	}

	// Force gophercloud to use the clouds.yaml file specified in our config
	if openstackConfig.CloudsFile != "" {
		if err := os.Setenv("OS_CLIENT_CONFIG_FILE", openstackConfig.CloudsFile); err != nil {
			return nil, err
		}
	}

	cloudConfig, rerr := clientconfig.GetCloudFromYAML(opts)
	if rerr != nil {
		return nil, rerr
	}

	// Initialize the underlying provider client
	provider, err := openstack.NewClient(cloudConfig.AuthInfo.AuthURL)
	if err != nil {
		return nil, err
	}
	provider.MaxBackoffRetries = 3 // default retries

	// Bypass TLS certificate validation if requested
	if cloudConfig.Verify != nil && !*cloudConfig.Verify {
		tlsconfig := &tls.Config{InsecureSkipVerify: true}

		var transport *http.Transport
		if defaultTransport, ok := http.DefaultTransport.(*http.Transport); ok {
			transport = defaultTransport.Clone()
		} else {
			transport = &http.Transport{Proxy: http.ProxyFromEnvironment}
		}
		transport.TLSClientConfig = tlsconfig
		provider.HTTPClient.Transport = transport
	}

	// Extract credentials and configure automatic token refreshing
	ao, err := clientconfig.AuthOptions(opts)
	if err != nil {
		return nil, err
	}
	ao.AllowReauth = true

	// Perform the initial token acquisition
	if aerr := openstack.Authenticate(authContext, provider, *ao); aerr != nil {
		return nil, aerr
	}

	var availability gophercloud.Availability
	switch cloudConfig.EndpointType {
	case "internal":
		availability = gophercloud.AvailabilityInternal
	case "admin":
		availability = gophercloud.AvailabilityAdmin
	default:
		availability = gophercloud.AvailabilityPublic
	}

	endpointOpts := gophercloud.EndpointOpts{
		Availability: availability,
		Region:       cloudConfig.RegionName,
	}

	blockStorage, err := openstack.NewBlockStorageV3(provider, endpointOpts)
	if err != nil {
		return nil, err
	}

	compute, err := openstack.NewComputeV2(provider, endpointOpts)
	if err != nil {
		return nil, err
	}

	identity, err := openstack.NewIdentityV3(provider, endpointOpts)
	if err != nil {
		return nil, err
	}

	c.BlockStorageClient = blockStorage
	c.ComputeClient = compute
	c.IdentityClient = identity
	c.Region = cloudConfig.RegionName
	c.Interface = cloudConfig.EndpointType

	return &c, nil
}
