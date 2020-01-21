package compute

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2019-07-01/compute"
	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-09-01/network"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/helpers/azure"
)

type connectionInfo struct {
	// primaryPrivateAddress is the Primary Private IP Address for this VM
	primaryPrivateAddress string

	// privateAddresses is a slice of the Private IP Addresses supported by this VM
	privateAddresses []string

	// primaryPublicAddress is the Primary Public IP Address for this VM
	primaryPublicAddress string

	// publicAddresses is a slice of the Public IP Addresses supported by this VM
	publicAddresses []string
}

// retrieveConnectionInformation retrieves all of the Public and Private IP Addresses assigned to a Virtual Machine
func retrieveConnectionInformation(ctx context.Context, client *network.InterfacesClient, input *compute.VirtualMachineProperties) connectionInfo {
	if input == nil || input.NetworkProfile == nil || input.NetworkProfile.NetworkInterfaces == nil {
		return connectionInfo{}
	}

	privateIPAddresses := make([]string, 0)
	publicIPAddresses := make([]string, 0)

	if input != nil && input.NetworkProfile != nil && input.NetworkProfile.NetworkInterfaces != nil {
		for _, v := range *input.NetworkProfile.NetworkInterfaces {
			if v.ID == nil {
				continue
			}

			nic := retrieveIPAddressesForNIC(ctx, client, *v.ID)
			if nic == nil {
				continue
			}

			privateIPAddresses = append(privateIPAddresses, nic.privateIPAddresses...)
			publicIPAddresses = append(publicIPAddresses, nic.publicIPAddresses...)
		}
	}

	primaryPrivateAddress := ""
	if len(privateIPAddresses) > 0 {
		primaryPrivateAddress = privateIPAddresses[0]
	}
	primaryPublicAddress := ""
	if len(publicIPAddresses) > 0 {
		primaryPublicAddress = publicIPAddresses[0]
	}

	return connectionInfo{
		primaryPrivateAddress: primaryPrivateAddress,
		privateAddresses:      privateIPAddresses,
		primaryPublicAddress:  primaryPublicAddress,
		publicAddresses:       publicIPAddresses,
	}
}

type interfaceDetails struct {
	// privateIPAddresses is a slice of the Private IP Addresses supported by this VM
	privateIPAddresses []string

	// publicIPAddresses is a slice of the Public IP Addresses supported by this VM
	publicIPAddresses []string
}

// retrieveIPAddressesForNIC returns the Public and Private IP Addresses associated
// with the specified Network Interface
func retrieveIPAddressesForNIC(ctx context.Context, client *network.InterfacesClient, nicID string) *interfaceDetails {
	id, err := azure.ParseAzureResourceID(nicID)
	if err != nil {
		return nil
	}

	resourceGroup := id.ResourceGroup
	name := id.Path["networkInterfaces"]

	nic, err := client.Get(ctx, resourceGroup, name, "")
	if err != nil {
		return nil
	}

	if nic.InterfacePropertiesFormat == nil || nic.InterfacePropertiesFormat.IPConfigurations == nil {
		return nil
	}

	privateIPAddresses := make([]string, 0)
	publicIPAddresses := make([]string, 0)
	for _, config := range *nic.InterfacePropertiesFormat.IPConfigurations {
		if props := config.InterfaceIPConfigurationPropertiesFormat; props != nil {

			if props.PrivateIPAddress != nil {
				privateIPAddresses = append(privateIPAddresses, *props.PrivateIPAddress)
			}

			if pip := props.PublicIPAddress; pip != nil {
				if pipProps := pip.PublicIPAddressPropertiesFormat; pipProps != nil {
					if pipProps.IPAddress != nil {
						publicIPAddresses = append(publicIPAddresses, *pipProps.IPAddress)
					}
				}
			}
		}
	}

	return &interfaceDetails{
		privateIPAddresses: privateIPAddresses,
		publicIPAddresses:  publicIPAddresses,
	}
}

// setConnectionInformation sets the connection information required for Provisioners
// to connect to the Virtual Machine. A Public IP Address is used if one is available
// but this falls back to a Private IP Address (which should always exist)
func setConnectionInformation(d *schema.ResourceData, input connectionInfo, isWindows bool) {
	provisionerType := "ssh"
	if isWindows {
		provisionerType = "winrm"
	}

	ipAddress := input.primaryPublicAddress
	if ipAddress == "" {
		ipAddress = input.primaryPrivateAddress
	}

	d.SetConnInfo(map[string]string{
		"type": provisionerType,
		"host": ipAddress,
	})
}
