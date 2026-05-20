package runtime

import (
	"context"
	"testing"

	"github.com/kubercloud/ani/pkg/ports"
)

func TestLocalNetworkServiceVPCDevProfile(t *testing.T) {
	service := NewLocalNetworkService()
	vpc, err := service.CreateVPC(context.Background(), ports.NetworkVPCCreateRequest{
		TenantID: "tenant-a",
		Name:     "tenant-a-vpc",
		CIDR:     "10.20.0.0/16",
	})
	if err != nil {
		t.Fatalf("CreateVPC error = %v", err)
	}
	if vpc.VPCID == "" || vpc.State != ports.NetworkResourceAvailable || vpc.CIDR != "10.20.0.0/16" {
		t.Fatalf("vpc = %+v, want available local VPC", vpc)
	}
	items, err := service.ListVPCs(context.Background(), ports.NetworkResourceListRequest{TenantID: "tenant-a"})
	if err != nil {
		t.Fatalf("ListVPCs error = %v", err)
	}
	if len(items) != 1 || items[0].VPCID != vpc.VPCID {
		t.Fatalf("tenant-a vpcs = %#v, want created vpc", items)
	}
	otherTenant, err := service.ListVPCs(context.Background(), ports.NetworkResourceListRequest{TenantID: "tenant-b"})
	if err != nil {
		t.Fatalf("ListVPCs(other tenant) error = %v", err)
	}
	if len(otherTenant) != 0 {
		t.Fatalf("tenant-b vpcs = %#v, want tenant isolation", otherTenant)
	}
}

func TestLocalNetworkServiceSubnetRequiresTenantVPC(t *testing.T) {
	service := NewLocalNetworkService()
	vpc, err := service.CreateVPC(context.Background(), ports.NetworkVPCCreateRequest{TenantID: "tenant-a", Name: "vpc-a"})
	if err != nil {
		t.Fatalf("CreateVPC error = %v", err)
	}
	subnet, err := service.CreateSubnet(context.Background(), ports.NetworkSubnetCreateRequest{
		TenantID: "tenant-a",
		VPCID:    vpc.VPCID,
		Name:     "subnet-a",
		CIDR:     "10.20.1.0/24",
		Gateway:  "10.20.1.1",
	})
	if err != nil {
		t.Fatalf("CreateSubnet error = %v", err)
	}
	if subnet.SubnetID == "" || subnet.VPCID != vpc.VPCID || subnet.State != ports.NetworkResourceAvailable {
		t.Fatalf("subnet = %+v, want available subnet under vpc", subnet)
	}
	if _, err := service.CreateSubnet(context.Background(), ports.NetworkSubnetCreateRequest{
		TenantID: "tenant-b",
		VPCID:    vpc.VPCID,
		Name:     "bad-subnet",
	}); err == nil {
		t.Fatalf("CreateSubnet with another tenant VPC succeeded, want error")
	}
}

func TestLocalNetworkServiceSecurityGroupAndLoadBalancer(t *testing.T) {
	service := NewLocalNetworkService()
	vpc, err := service.CreateVPC(context.Background(), ports.NetworkVPCCreateRequest{TenantID: "tenant-a", Name: "vpc-a"})
	if err != nil {
		t.Fatalf("CreateVPC error = %v", err)
	}
	sg, err := service.CreateSecurityGroup(context.Background(), ports.NetworkSecurityGroupCreateRequest{
		TenantID: "tenant-a",
		Name:     "web-sg",
		Rules: []ports.NetworkSecurityGroupRule{
			{Direction: "ingress", Protocol: "tcp", PortRange: "443", CIDR: "0.0.0.0/0", Action: "allow"},
		},
	})
	if err != nil {
		t.Fatalf("CreateSecurityGroup error = %v", err)
	}
	if sg.SecurityGroupID == "" || len(sg.Rules) != 1 {
		t.Fatalf("security group = %+v, want one rule", sg)
	}
	lb, err := service.CreateLoadBalancer(context.Background(), ports.NetworkLoadBalancerCreateRequest{
		TenantID: "tenant-a",
		Name:     "web-lb",
		VPCID:    vpc.VPCID,
		Scheme:   "public",
		Listeners: []ports.NetworkLoadBalancerListener{
			{Protocol: "http", Port: 80, TargetPort: 8080},
		},
	})
	if err != nil {
		t.Fatalf("CreateLoadBalancer error = %v", err)
	}
	if lb.LoadBalancerID == "" || lb.VIP == "" || lb.State != ports.NetworkResourceAvailable {
		t.Fatalf("load balancer = %+v, want available local lb", lb)
	}
	deleted, err := service.DeleteLoadBalancer(context.Background(), ports.NetworkResourceGetRequest{
		TenantID:   "tenant-a",
		ResourceID: lb.LoadBalancerID,
	})
	if err != nil {
		t.Fatalf("DeleteLoadBalancer error = %v", err)
	}
	if deleted.State != ports.NetworkResourceDeleted {
		t.Fatalf("deleted lb state = %s, want deleted", deleted.State)
	}
	list, err := service.ListLoadBalancers(context.Background(), ports.NetworkResourceListRequest{TenantID: "tenant-a"})
	if err != nil {
		t.Fatalf("ListLoadBalancers error = %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("load balancers = %#v, want deleted item hidden", list)
	}
}
