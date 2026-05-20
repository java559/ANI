package router

import (
	"context"
	"testing"

	"github.com/kubercloud/ani/pkg/ports"
)

func TestNetworkAPIDevProfileVPCSubnetSecurityGroupAndLB(t *testing.T) {
	api := newNetworkAPI()
	vpc, err := api.service.CreateVPC(context.Background(), ports.NetworkVPCCreateRequest{
		TenantID: "tenant-a",
		Name:     "tenant-a-vpc",
		CIDR:     "10.30.0.0/16",
	})
	if err != nil {
		t.Fatalf("CreateVPC error = %v", err)
	}
	if got := networkVPCFromRecord(vpc); got.ID == "" || got.State != "available" || got.TenantID != "tenant-a" {
		t.Fatalf("vpc response = %+v, want available tenant-a VPC", got)
	} else {
		requireLocalCoreDevProfile(t, got.DevProfile, "local-network-service")
	}
	subnet, err := api.service.CreateSubnet(context.Background(), ports.NetworkSubnetCreateRequest{
		TenantID: "tenant-a",
		VPCID:    vpc.VPCID,
		Name:     "tenant-a-subnet",
		CIDR:     "10.30.1.0/24",
	})
	if err != nil {
		t.Fatalf("CreateSubnet error = %v", err)
	}
	if got := networkSubnetFromRecord(subnet); got.ID == "" || got.VPCID != vpc.VPCID || got.State != "available" {
		t.Fatalf("subnet response = %+v, want subnet under VPC", got)
	} else {
		requireLocalCoreDevProfile(t, got.DevProfile, "local-network-service")
	}
	sg, err := api.service.CreateSecurityGroup(context.Background(), ports.NetworkSecurityGroupCreateRequest{
		TenantID: "tenant-a",
		Name:     "web-sg",
		Rules: []ports.NetworkSecurityGroupRule{
			{Direction: "ingress", Protocol: "tcp", PortRange: "443", CIDR: "0.0.0.0/0", Action: "allow"},
		},
	})
	if err != nil {
		t.Fatalf("CreateSecurityGroup error = %v", err)
	}
	if got := networkSecurityGroupFromRecord(sg); got.ID == "" || len(got.Rules) != 1 {
		t.Fatalf("security group response = %+v, want rule", got)
	} else {
		requireLocalCoreDevProfile(t, got.DevProfile, "local-network-service")
	}
	lb, err := api.service.CreateLoadBalancer(context.Background(), ports.NetworkLoadBalancerCreateRequest{
		TenantID: "tenant-a",
		Name:     "web-lb",
		VPCID:    vpc.VPCID,
		Listeners: []ports.NetworkLoadBalancerListener{
			{Protocol: "http", Port: 80, TargetPort: 8080},
		},
	})
	if err != nil {
		t.Fatalf("CreateLoadBalancer error = %v", err)
	}
	if got := networkLoadBalancerFromRecord(lb); got.ID == "" || got.VPCID != vpc.VPCID || got.VIP == "" {
		t.Fatalf("load balancer response = %+v, want local lb", got)
	} else {
		requireLocalCoreDevProfile(t, got.DevProfile, "local-network-service")
	}
}

func TestNetworkAPIServiceKeepsTenantIsolation(t *testing.T) {
	api := newNetworkAPI()
	vpc, err := api.service.CreateVPC(context.Background(), ports.NetworkVPCCreateRequest{
		TenantID: "tenant-a",
		Name:     "tenant-a-vpc",
	})
	if err != nil {
		t.Fatalf("CreateVPC error = %v", err)
	}
	if _, err := api.service.GetVPC(context.Background(), ports.NetworkResourceGetRequest{
		TenantID:   "tenant-b",
		ResourceID: vpc.VPCID,
	}); err == nil {
		t.Fatalf("GetVPC from another tenant succeeded, want isolation error")
	}
}
