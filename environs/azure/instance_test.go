// Copyright 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package azure

import (
	"encoding/base64"

	. "launchpad.net/gocheck"
	"launchpad.net/gwacl"

	"launchpad.net/juju-core/instance"
	"net/http"
)

type InstanceSuite struct{}

var _ = Suite(new(InstanceSuite))

// makeHostedServiceDescriptor creates a HostedServiceDescriptor with the
// given service name.
func makeHostedServiceDescriptor(name string) *gwacl.HostedServiceDescriptor {
	labelBase64 := base64.StdEncoding.EncodeToString([]byte("label"))
	return &gwacl.HostedServiceDescriptor{ServiceName: name, Label: labelBase64}
}

func (*StorageSuite) TestId(c *C) {
	serviceName := "test-name"
	testService := makeHostedServiceDescriptor(serviceName)
	azInstance := azureInstance{*testService, nil}
	c.Check(azInstance.Id(), Equals, instance.Id(serviceName))
}

func (*StorageSuite) TestDNSName(c *C) {
	// An instance's DNS name is computed from its hosted-service name.
	host := "hostname"
	testService := makeHostedServiceDescriptor(host)
	azInstance := azureInstance{*testService, nil}
	dnsName, err := azInstance.DNSName()
	c.Assert(err, IsNil)
	c.Check(dnsName, Equals, host+"."+AZURE_DOMAIN_NAME)
}

func (*StorageSuite) TestWaitDNSName(c *C) {
	// An Azure instance gets its DNS name immediately, so there's no
	// waiting involved.
	host := "hostname"
	testService := makeHostedServiceDescriptor(host)
	azInstance := azureInstance{*testService, nil}
	dnsName, err := azInstance.WaitDNSName()
	c.Assert(err, IsNil)
	c.Check(dnsName, Equals, host+"."+AZURE_DOMAIN_NAME)
}

func (*StorageSuite) TestOpenPorts(c *C) {
	service := makeHostedServiceDescriptor("host-name")
	deployments := []gwacl.Deployment{
		{
			Name: "deployment-one",
			RoleList: []gwacl.Role{
				{RoleName: "role-one"},
				{RoleName: "role-two"},
			},
		},
		{
			Name: "deployment-two",
			RoleList: []gwacl.Role{
				{RoleName: "role-three"},
			},
		},
	}
	hostedService := &gwacl.HostedService{
		Deployments:             deployments,
		HostedServiceDescriptor: *service,
		XMLNS: gwacl.XMLNS,
	}
	serialize := func(object gwacl.AzureObject) []byte {
		xml, err := object.Serialize()
		c.Assert(err, IsNil)
		return []byte(xml)
	}
	// First, GetHostedServiceProperties
	responses := []gwacl.DispatcherResponse{
		gwacl.NewDispatcherResponse(
			serialize(hostedService), http.StatusOK, nil),
	}
	// Then, GetRole and UpdateRole for each role.
	for _, deployment := range deployments {
		for _, role := range deployment.RoleList {
			// GetRole returns a PersistentVMRole.
			persistentRole := &gwacl.PersistentVMRole{
				XMLNS:    gwacl.XMLNS,
				RoleName: role.RoleName,
			}
			response := gwacl.NewDispatcherResponse(
				serialize(persistentRole), http.StatusOK, nil)
			responses = append(responses, response)
			// UpdateRole expects a 200 response, that's all.
			responses = append(responses,
				gwacl.NewDispatcherResponse(nil, http.StatusOK, nil))
		}
	}
	record := gwacl.PatchManagementAPIResponses(responses)
	azInstance := azureInstance{*service, makeEnviron(c)}

	err := azInstance.OpenPorts("machine-id", []instance.Port{
		{"finger", 79}, {"submission", 587}, {"gopher", 70},
	})

	c.Assert(err, IsNil)
	c.Assert(*record, HasLen, 7)
}
