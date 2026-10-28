package provider

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"regexp"
	"sync"
	"testing"

	"github.com/flyteorg/flyte/flyteidl/gen/pb-go/flyteidl/core"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/unionai/cloud/gen/pb-go/cluster"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"
)

// fakeManagedClusterServer implements GetNodeGroups and SetNodeGroups the way the control plane
// does: writes replace the whole set, the generation moves only when the content changes, and a
// stale expected_generation is refused with Aborted.
type fakeManagedClusterServer struct {
	cluster.UnimplementedManagedClusterServiceServer

	mu       sync.Mutex
	clusters map[string]*fakeCluster
	// beforeSet, when set, runs once inside the next SetNodeGroups, to simulate an edit that
	// lands between Terraform's read and its write.
	beforeSet func(c *fakeCluster)
}

type fakeCluster struct {
	groups     []*cluster.NodeGroup
	generation uint64
	byoc       bool
}

func (s *fakeManagedClusterServer) lookup(org, name string) (*fakeCluster, error) {
	c, ok := s.clusters[org+"/"+name]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "cluster %s/%s not found", org, name)
	}
	return c, nil
}

func (s *fakeManagedClusterServer) GetNodeGroups(ctx context.Context, req *cluster.GetNodeGroupsRequest) (*cluster.GetNodeGroupsResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, err := s.lookup(req.GetId().GetOrg().GetName(), req.GetId().GetName())
	if err != nil {
		return nil, err
	}
	return &cluster.GetNodeGroupsResponse{NodeGroups: cloneNodeGroups(c.groups), Generation: c.generation}, nil
}

func (s *fakeManagedClusterServer) SetNodeGroups(ctx context.Context, req *cluster.SetNodeGroupsRequest) (*cluster.SetNodeGroupsResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, err := s.lookup(req.GetId().GetOrg().GetName(), req.GetId().GetName())
	if err != nil {
		return nil, err
	}
	if c.byoc {
		return nil, status.Error(codes.PermissionDenied, "node groups of a Union-provisioned cluster are managed by Union")
	}
	if s.beforeSet != nil {
		s.beforeSet(c)
		s.beforeSet = nil
	}
	if req.GetExpectedGeneration() != 0 && req.GetExpectedGeneration() != c.generation {
		return nil, status.Errorf(codes.Aborted, "expected generation %d, cluster is at %d", req.GetExpectedGeneration(), c.generation)
	}
	if !equalNodeGroups(c.groups, req.GetNodeGroups()) {
		c.generation++
	}
	c.groups = cloneNodeGroups(req.GetNodeGroups())
	return &cluster.SetNodeGroupsResponse{Generation: c.generation}, nil
}

func (s *fakeManagedClusterServer) groups(key string) []*cluster.NodeGroup {
	s.mu.Lock()
	defer s.mu.Unlock()
	return cloneNodeGroups(s.clusters[key].groups)
}

func cloneNodeGroups(groups []*cluster.NodeGroup) []*cluster.NodeGroup {
	cloned := make([]*cluster.NodeGroup, 0, len(groups))
	for _, group := range groups {
		cloned = append(cloned, proto.Clone(group).(*cluster.NodeGroup))
	}
	return cloned
}

func equalNodeGroups(left, right []*cluster.NodeGroup) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if !proto.Equal(left[i], right[i]) {
			return false
		}
	}
	return true
}

// nodeGroupsTestProvider is the real provider with Configure swapped out, so it talks to an
// in-process fake instead of a control plane.
type nodeGroupsTestProvider struct {
	UnionaiProvider
	conn *grpc.ClientConn
}

func (p *nodeGroupsTestProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	client := &providerContext{conn: p.conn, org: "acme"}
	resp.ResourceData = client
	resp.DataSourceData = client
}

func (p *nodeGroupsTestProvider) Resources(ctx context.Context) []func() frameworkresource.Resource {
	return []func() frameworkresource.Resource{NewNodeGroupsResource}
}

func (p *nodeGroupsTestProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{NewNodeGroupsDataSource}
}

// newNodeGroupsTestServer starts the fake with a customer-managed cluster `acme/prod` and a
// BYOC cluster `acme/byoc`, and returns provider factories wired to it.
func newNodeGroupsTestServer(t *testing.T) (*fakeManagedClusterServer, map[string]func() (tfprotov6.ProviderServer, error)) {
	t.Helper()
	if _, err := exec.LookPath("terraform"); err != nil {
		t.Skip("terraform CLI not found in PATH")
	}

	fake := &fakeManagedClusterServer{clusters: map[string]*fakeCluster{
		"acme/prod": {},
		"acme/byoc": {byoc: true},
	}}

	listener := bufconn.Listen(1 << 20)
	server := grpc.NewServer()
	cluster.RegisterManagedClusterServiceServer(server, fake)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return listener.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial fake server: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	factories := map[string]func() (tfprotov6.ProviderServer, error){
		"unionai": providerserver.NewProtocol6WithError(&nodeGroupsTestProvider{UnionaiProvider: UnionaiProvider{version: "test"}, conn: conn}),
	}
	return fake, factories
}

const nodeGroupCPU = `
  node_group {
    name                  = "cpu-general"
    min_nodes             = 0
    spot_instances        = false
    node_disk_size_gbs    = 100
    max_nodes             = 10
    node_instance_types   = ["m6i.2xlarge"]
    capacity_types        = ["ON_DEMAND"]
    labels                = { workload = "general" }
    allocatable_resources = { cpu = "8", memory = "32Gi" }
  }
`

func nodeGroupGPU(maxNodes int) string {
	return fmt.Sprintf(`
  node_group {
    name                = "gpu-a100"
    max_nodes           = %d
    node_disk_size_gbs  = 500
    node_instance_types = ["p4d.24xlarge"]
    capacity_types      = ["SPOT", "ON_DEMAND"]
    architecture        = "AMD64"
    weight              = 50
    max_pods            = 110
    limits              = { "nvidia.com/gpu" = "32" }

    taint {
      key    = "nvidia.com/gpu"
      value  = "present"
      effect = "NoSchedule"
    }

    taint {
      key    = "dedicated"
      effect = "NoExecute"
    }

    accelerator {
      device       = "A100"
      manufacturer = "NVIDIA"
      count        = 8
    }
  }
`, maxNodes)
}

func nodeGroupsConfig(clusterName string, groups ...string) string {
	config := fmt.Sprintf("resource \"unionai_node_groups\" \"test\" {\n  cluster = %q\n", clusterName)
	for _, group := range groups {
		config += group
	}
	return config + "}\n"
}

func TestNodeGroupsResource_Lifecycle(t *testing.T) {
	fake, factories := newNodeGroupsTestServer(t)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		CheckDestroy: func(*terraform.State) error {
			if groups := fake.groups("acme/prod"); len(groups) != 0 {
				return fmt.Errorf("expected destroy to leave no node groups, found %d", len(groups))
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: nodeGroupsConfig("prod", nodeGroupCPU, nodeGroupGPU(4)),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("unionai_node_groups.test", "id", "acme/prod"),
					resource.TestCheckResourceAttr("unionai_node_groups.test", "org", "acme"),
					resource.TestCheckResourceAttr("unionai_node_groups.test", "generation", "1"),
					resource.TestCheckResourceAttr("unionai_node_groups.test", "node_group.#", "2"),
					resource.TestCheckTypeSetElemNestedAttrs("unionai_node_groups.test", "node_group.*", map[string]string{
						"name":           "cpu-general",
						"min_nodes":      "0",
						"spot_instances": "false",
						"taint.#":        "0",
					}),
					resource.TestCheckTypeSetElemNestedAttrs("unionai_node_groups.test", "node_group.*", map[string]string{
						"name":    "gpu-a100",
						"taint.#": "2",
					}),
					func(*terraform.State) error {
						groups := fake.groups("acme/prod")
						if len(groups) != 2 || groups[0].GetName() != "cpu-general" || groups[1].GetName() != "gpu-a100" {
							return fmt.Errorf("unexpected groups on server: %v", groups)
						}
						gpu := groups[1]
						if gpu.GetTaints()[0].GetEffect() != cluster.TaintEffect_NO_SCHEDULE ||
							gpu.GetWeight() != 50 ||
							gpu.GetArchitecture() != cluster.NodeArchitecture_NODE_ARCHITECTURE_AMD64 ||
							gpu.GetCapacityTypes()[0] != cluster.CapacityType_CAPACITY_TYPE_SPOT ||
							gpu.GetAccelerators()[0].GetCount() != 8 {
							return fmt.Errorf("gpu group stored wrongly: %v", gpu)
						}
						if groups[0].Weight != nil {
							return fmt.Errorf("unset weight was sent as %d", groups[0].GetWeight())
						}
						return nil
					},
				),
			},
			// Reordering the blocks is not a change.
			{
				Config:   nodeGroupsConfig("prod", nodeGroupGPU(4), nodeGroupCPU),
				PlanOnly: true,
			},
			// Dropping a group deletes it; changing one moves the generation.
			{
				Config: nodeGroupsConfig("prod", nodeGroupGPU(8)),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("unionai_node_groups.test", "generation", "2"),
					resource.TestCheckResourceAttr("unionai_node_groups.test", "node_group.#", "1"),
					func(*terraform.State) error {
						groups := fake.groups("acme/prod")
						if len(groups) != 1 || groups[0].GetMaxNodes() != 8 {
							return fmt.Errorf("unexpected groups on server: %v", groups)
						}
						return nil
					},
				),
			},
			// An edit that lands between refresh and apply makes the write fail rather than clobber it.
			{
				PreConfig: func() {
					fake.mu.Lock()
					fake.beforeSet = func(c *fakeCluster) { c.generation++ }
					fake.mu.Unlock()
				},
				Config:      nodeGroupsConfig("prod", nodeGroupGPU(6)),
				ExpectError: regexp.MustCompile(`Node groups changed outside Terraform`),
			},
			// The next run refreshes the generation and goes through.
			{
				Config: nodeGroupsConfig("prod", nodeGroupGPU(6)),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("unionai_node_groups.test", "generation", "4"),
				),
			},
			{
				ResourceName:      "unionai_node_groups.test",
				ImportState:       true,
				ImportStateId:     "acme/prod",
				ImportStateVerify: true,
			},
			{
				ResourceName:      "unionai_node_groups.test",
				ImportState:       true,
				ImportStateId:     "prod",
				ImportStateVerify: true,
			},
			{
				Config: nodeGroupsConfig("prod", nodeGroupGPU(6)) + `
data "unionai_node_groups" "test" {
  cluster = "prod"
  depends_on = [unionai_node_groups.test]
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.unionai_node_groups.test", "id", "acme/prod"),
					resource.TestCheckResourceAttr("data.unionai_node_groups.test", "generation", "4"),
					resource.TestCheckResourceAttr("data.unionai_node_groups.test", "node_group.0.name", "gpu-a100"),
					resource.TestCheckResourceAttr("data.unionai_node_groups.test", "node_group.0.taint.0.effect", "NoSchedule"),
					resource.TestCheckResourceAttr("data.unionai_node_groups.test", "node_group.0.accelerator.0.device", "A100"),
					resource.TestCheckResourceAttr("data.unionai_node_groups.test", "node_group.0.limits.nvidia.com/gpu", "32"),
				),
			},
		},
	})
}

// Drift made outside Terraform shows up in the plan.
func TestNodeGroupsResource_DetectsDrift(t *testing.T) {
	fake, factories := newNodeGroupsTestServer(t)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				Config: nodeGroupsConfig("prod", nodeGroupCPU),
			},
			{
				PreConfig: func() {
					fake.mu.Lock()
					defer fake.mu.Unlock()
					c := fake.clusters["acme/prod"]
					c.groups[0].MaxNodes = 99
					c.generation++
				},
				Config:             nodeGroupsConfig("prod", nodeGroupCPU),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
			// Applying puts the configuration back. It also refreshes the stored generation, which
			// the post-test destroy (run without a refresh) needs.
			{
				Config: nodeGroupsConfig("prod", nodeGroupCPU),
				Check: func(*terraform.State) error {
					if groups := fake.groups("acme/prod"); groups[0].GetMaxNodes() != 10 {
						return fmt.Errorf("max_nodes = %d, want 10", groups[0].GetMaxNodes())
					}
					return nil
				},
			},
		},
	})
}

func TestNodeGroupsResource_BYOCClusterRefused(t *testing.T) {
	_, factories := newNodeGroupsTestServer(t)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				Config:      nodeGroupsConfig("byoc", nodeGroupCPU),
				ExpectError: regexp.MustCompile(`Not permitted to manage node groups`),
			},
		},
	})
}

func TestNodeGroupsResource_ValidatesAtPlan(t *testing.T) {
	_, factories := newNodeGroupsTestServer(t)

	group := func(body string) string {
		return fmt.Sprintf(`
  node_group {
    node_disk_size_gbs  = 100
    node_instance_types = ["m6i.2xlarge"]
    %s
  }
`, body)
	}

	cases := map[string]struct {
		groups []string
		err    string
	}{
		"bad name":          {[]string{group(`name = "1-bad"`)}, `Invalid node group name`},
		"duplicate name":    {[]string{group(`name = "a"`), group(`name = "a"` + "\n    max_nodes = 3")}, `Duplicate node group name`},
		"weight":            {[]string{group("name = \"a\"\n    weight = 0")}, `weight must be between 1 and 100`},
		"capacity type":     {[]string{group("name = \"a\"\n    capacity_types = [\"PREEMPTIBLE\"]")}, `capacity_types entries must be one of`},
		"architecture":      {[]string{group("name = \"a\"\n    architecture = \"x86\"")}, `architecture must be one of`},
		"bounds":            {[]string{group("name = \"a\"\n    min_nodes = 5\n    max_nodes = 2")}, `min_nodes \(5\) is greater than max_nodes`},
		"disk size":         {[]string{"\n  node_group {\n    name = \"a\"\n    node_disk_size_gbs = 0\n    node_instance_types = [\"m\"]\n  }\n"}, `node_disk_size_gbs must be between 1`},
		"no instance types": {[]string{"\n  node_group {\n    name = \"a\"\n    node_disk_size_gbs = 1\n    node_instance_types = []\n  }\n"}, `at least one instance type`},
		"taint effect":      {[]string{group("name = \"a\"\n    taint {\n      key = \"k\"\n      effect = \"NO_SCHEDULE\"\n    }")}, `taint effect must be one of`},
		"resource name":     {[]string{group("name = \"a\"\n    allocatable_resources = { CPU = \"1\" }")}, `allocatable_resources keys must be one of`},
		"accelerator count": {[]string{group("name = \"a\"\n    accelerator {\n      device = \"T4\"\n      manufacturer = \"NVIDIA\"\n      count = 0\n    }")}, `accelerator count must be greater than 0`},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			resource.UnitTest(t, resource.TestCase{
				ProtoV6ProviderFactories: factories,
				Steps: []resource.TestStep{
					{
						Config:      nodeGroupsConfig("prod", tc.groups...),
						PlanOnly:    true,
						ExpectError: regexp.MustCompile(tc.err),
					},
				},
			})
		})
	}
}

func TestNodeGroupConversionRoundTrip(t *testing.T) {
	weight := int32(20)
	original := &cluster.NodeGroup{
		Name:              "gpu-a100",
		MinNodes:          1,
		MaxNodes:          4,
		NodeDiskSizeGbs:   500,
		NodeInstanceTypes: []string{"p4d.24xlarge", "p5.48xlarge"},
		SpotInstances:     true,
		AllocatableResources: []*core.Resources_ResourceEntry{
			{Name: core.Resources_CPU, Value: "96"},
			{Name: core.Resources_EPHEMERAL_STORAGE, Value: "1Ti"},
			{Name: core.Resources_GPU, Value: "8"},
		},
		Labels: map[string]string{"workload": "gpu"},
		Taints: []*cluster.Taint{
			{Key: "nvidia.com/gpu", Value: "present", Effect: cluster.TaintEffect_NO_SCHEDULE},
			{Key: "dedicated", Effect: cluster.TaintEffect_PREFER_NO_SCHEDULE},
		},
		Accelerators: []*cluster.NodeGroupAccelerator{
			{Device: "A100", Manufacturer: "NVIDIA", Count: 8, PartitionSize: "1g.5gb"},
		},
		Architecture:  cluster.NodeArchitecture_NODE_ARCHITECTURE_ARM64,
		CapacityTypes: []cluster.CapacityType{cluster.CapacityType_CAPACITY_TYPE_SPOT, cluster.CapacityType_CAPACITY_TYPE_RESERVED},
		Weight:        &weight,
		MaxPods:       110,
		Limits:        map[string]string{"cpu": "1000", "nvidia.com/gpu": "32"},
	}

	ctx := context.Background()
	model, diags := nodeGroupToModel(ctx, original)
	if diags.HasError() {
		t.Fatalf("to model: %v", diags)
	}
	if got := validateNodeGroup(ctx, model); got.HasError() {
		t.Fatalf("a stored group failed validation: %v", got)
	}
	roundTripped, diags := nodeGroupFromModel(ctx, model)
	if diags.HasError() {
		t.Fatalf("from model: %v", diags)
	}
	if !proto.Equal(original, roundTripped) {
		t.Errorf("round trip changed the group:\n got: %v\nwant: %v", roundTripped, original)
	}
}

// A group read with no prior (an import) has zero values as null, so a configuration that omits
// them plans no change; one whose prior set them explicitly keeps them.
func TestFitNodeGroupToPrior(t *testing.T) {
	ctx := context.Background()
	model, diags := nodeGroupToModel(ctx, &cluster.NodeGroup{
		Name:   "a",
		Taints: []*cluster.Taint{{Key: "k", Effect: cluster.TaintEffect_NO_SCHEDULE}},
	})
	if diags.HasError() {
		t.Fatalf("to model: %v", diags)
	}
	if model.Taints == nil || model.Accelerators == nil {
		t.Error("taints and accelerators must be empty, not nil")
	}

	fitted := fitNodeGroupToPrior(model, nodeGroupModel{})
	for name, value := range map[string]attr.Value{
		"min_nodes": fitted.MinNodes, "max_nodes": fitted.MaxNodes, "max_pods": fitted.MaxPods,
		"spot_instances": fitted.SpotInstances, "capacity_types": fitted.CapacityTypes, "labels": fitted.Labels,
		"limits": fitted.Limits, "allocatable_resources": fitted.AllocatableResources,
		"weight": fitted.Weight, "architecture": fitted.Architecture, "taint.value": fitted.Taints[0].Value,
	} {
		if !value.IsNull() {
			t.Errorf("%s = %v, want null", name, value)
		}
	}

	emptyMap, _ := types.MapValue(types.StringType, nil)
	prior := nodeGroupModel{
		MinNodes:      types.Int64Value(0),
		SpotInstances: types.BoolValue(false),
		Labels:        emptyMap,
		Taints:        []taintModel{{Value: types.StringValue("")}},
	}
	fitted = fitNodeGroupToPrior(model, prior)
	if fitted.MinNodes.IsNull() || fitted.SpotInstances.IsNull() || fitted.Labels.IsNull() || fitted.Taints[0].Value.IsNull() {
		t.Errorf("explicit zero values in prior were not kept: %+v", fitted)
	}
	if !fitted.MaxNodes.IsNull() {
		t.Errorf("max_nodes = %v, want null", fitted.MaxNodes)
	}
}

// TestAccNodeGroupsResource runs against a live control plane. It replaces every node group on
// the cluster named by UNIONAI_TEST_NODE_GROUPS_CLUSTER, which must be customer-managed, and
// leaves it with none, so point it at a cluster set aside for testing.
func TestAccNodeGroupsResource(t *testing.T) {
	clusterName := os.Getenv("UNIONAI_TEST_NODE_GROUPS_CLUSTER")
	if clusterName == "" {
		t.Skip("UNIONAI_TEST_NODE_GROUPS_CLUSTER must be set to a customer-managed test cluster")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: nodeGroupsConfig(clusterName, nodeGroupCPU, nodeGroupGPU(4)),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("unionai_node_groups.test", "node_group.#", "2"),
					resource.TestCheckResourceAttrSet("unionai_node_groups.test", "generation"),
				),
			},
			{
				Config:   nodeGroupsConfig(clusterName, nodeGroupGPU(4), nodeGroupCPU),
				PlanOnly: true,
			},
			{
				Config: nodeGroupsConfig(clusterName, nodeGroupGPU(8)),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("unionai_node_groups.test", "node_group.#", "1"),
				),
			},
			{
				ResourceName:      "unionai_node_groups.test",
				ImportState:       true,
				ImportStateId:     clusterName,
				ImportStateVerify: true,
			},
		},
	})
}
