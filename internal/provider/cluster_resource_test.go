package provider

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"regexp"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/unionai/cloud/gen/pb-go/cluster"
	"github.com/unionai/cloud/gen/pb-go/common"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

// fakeClusterServer implements the cluster RPCs the resource uses the way the control plane
// documents them: names are shared with queues, a soft-deleted cluster reads as NotFound, a pool
// move needs the co-named queue drained, and deletes carry the generation last read.
type fakeClusterServer struct {
	cluster.UnimplementedClusterServiceServer

	mu       sync.Mutex
	clusters map[string]*fakeClusterRecord
	// queues holds names taken in the queue namespace by queues that are not cluster-owned.
	queues map[string]bool
	pools  map[string]bool
}

type fakeClusterRecord struct {
	pool         string
	state        cluster.ClusterState
	generation   uint64
	queueDrained bool
}

func (s *fakeClusterServer) CreateCluster(ctx context.Context, req *cluster.CreateClusterRequest) (*cluster.CreateClusterResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := req.GetClusterId().GetOrganization() + "/" + req.GetClusterId().GetName()
	if _, ok := s.clusters[key]; ok || s.queues[key] {
		return nil, status.Errorf(codes.AlreadyExists, "%s already exists", key)
	}
	pool := req.GetClusterPoolName()
	if pool == "" {
		pool = defaultClusterPoolName
	}
	if !s.pools[pool] {
		return nil, status.Errorf(codes.NotFound, "cluster pool %s not found", pool)
	}
	s.clusters[key] = &fakeClusterRecord{pool: pool, state: cluster.ClusterState_CLUSTER_STATE_ACTIVE, generation: 1}
	return &cluster.CreateClusterResponse{}, nil
}

func (s *fakeClusterServer) live(id *common.ClusterIdentifier) (*fakeClusterRecord, error) {
	record, ok := s.clusters[id.GetOrganization()+"/"+id.GetName()]
	if !ok || record.state == cluster.ClusterState_CLUSTER_STATE_DELETED {
		return nil, status.Errorf(codes.NotFound, "missing entity of type Cluster with identifier %v", id)
	}
	return record, nil
}

func (s *fakeClusterServer) toProto(id *common.ClusterIdentifier, record *fakeClusterRecord) *cluster.Cluster {
	return &cluster.Cluster{
		Pool:        &common.ClusterPoolIdentifier{Organization: id.GetOrganization(), Name: record.pool},
		DrainStatus: &cluster.ClusterDrainStatus{OverallState: record.state, Generation: record.generation},
	}
}

func (s *fakeClusterServer) GetCluster(ctx context.Context, req *cluster.GetRequest) (*cluster.GetResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, err := s.live(req.GetClusterId())
	if err != nil {
		return nil, err
	}
	return &cluster.GetResponse{Cluster: s.toProto(req.GetClusterId(), record)}, nil
}

func (s *fakeClusterServer) UpdateCluster(ctx context.Context, req *cluster.UpdateClusterRequest) (*cluster.UpdateClusterResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, err := s.live(req.GetClusterId())
	if err != nil {
		return nil, err
	}
	if record.pool == req.GetClusterPoolName() {
		return nil, status.Error(codes.AlreadyExists, "cluster is already in that pool")
	}
	if !s.pools[req.GetClusterPoolName()] {
		return nil, status.Errorf(codes.NotFound, "cluster pool %s not found", req.GetClusterPoolName())
	}
	if !record.queueDrained {
		return nil, status.Error(codes.FailedPrecondition, "the cluster's queue must be drained")
	}
	record.pool = req.GetClusterPoolName()
	return &cluster.UpdateClusterResponse{}, nil
}

func (s *fakeClusterServer) DeleteCluster(ctx context.Context, req *cluster.DeleteRequest) (*cluster.DeleteResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, err := s.live(req.GetClusterId())
	if err != nil {
		return nil, err
	}
	if req.GetExpectedGeneration() == 0 {
		return nil, status.Error(codes.InvalidArgument, "expected_generation is required")
	}
	if req.GetExpectedGeneration() != record.generation {
		return nil, status.Errorf(codes.FailedPrecondition, "expected generation %d, cluster is at %d", req.GetExpectedGeneration(), record.generation)
	}
	if record.state == cluster.ClusterState_CLUSTER_STATE_DRAINED {
		record.state = cluster.ClusterState_CLUSTER_STATE_DELETED
	} else {
		record.state = cluster.ClusterState_CLUSTER_STATE_DELETING
	}
	record.generation++
	return &cluster.DeleteResponse{Cluster: s.toProto(req.GetClusterId(), record)}, nil
}

func (s *fakeClusterServer) record(key string) *fakeClusterRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	if record, ok := s.clusters[key]; ok {
		copied := *record
		return &copied
	}
	return nil
}

// clusterTestProvider is the real provider with Configure swapped out, so it talks to an
// in-process fake instead of a control plane.
type clusterTestProvider struct {
	UnionaiProvider
	conn        *grpc.ClientConn
	allowedOrgs []string
}

func (p *clusterTestProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	client := &providerContext{conn: p.conn, org: "acme", allowedOrgs: p.allowedOrgs}
	resp.ResourceData = client
	resp.DataSourceData = client
}

func (p *clusterTestProvider) Resources(ctx context.Context) []func() frameworkresource.Resource {
	return []func() frameworkresource.Resource{NewClusterResource}
}

func (p *clusterTestProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return nil
}

// newClusterTestServer starts the fake with pools `default` and `gpu`, and a standalone queue
// `acme/batch` that holds its name in the queue namespace.
func newClusterTestServer(t *testing.T, allowedOrgs ...string) (*fakeClusterServer, map[string]func() (tfprotov6.ProviderServer, error)) {
	t.Helper()
	if _, err := exec.LookPath("terraform"); err != nil {
		t.Skip("terraform CLI not found in PATH")
	}

	fake := &fakeClusterServer{
		clusters: map[string]*fakeClusterRecord{},
		queues:   map[string]bool{"acme/batch": true},
		pools:    map[string]bool{defaultClusterPoolName: true, "gpu": true},
	}

	listener := bufconn.Listen(1 << 20)
	server := grpc.NewServer()
	cluster.RegisterClusterServiceServer(server, fake)
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
		"unionai": providerserver.NewProtocol6WithError(&clusterTestProvider{UnionaiProvider: UnionaiProvider{version: "test"}, conn: conn, allowedOrgs: allowedOrgs}),
	}
	return fake, factories
}

func clusterConfig(name, pool string) string {
	config := fmt.Sprintf("resource \"unionai_cluster\" \"test\" {\n  name = %q\n", name)
	if pool != "" {
		config += fmt.Sprintf("  cluster_pool_name = %q\n", pool)
	}
	return config + "}\n"
}

func TestClusterResource_Lifecycle(t *testing.T) {
	fake, factories := newClusterTestServer(t)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		CheckDestroy: func(*terraform.State) error {
			record := fake.record("acme/prod")
			if record == nil || record.state != cluster.ClusterState_CLUSTER_STATE_DELETING {
				return fmt.Errorf("expected destroy to leave acme/prod deleting, got %+v", record)
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: clusterConfig("prod", ""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("unionai_cluster.test", "id", "acme/prod"),
					resource.TestCheckResourceAttr("unionai_cluster.test", "org", "acme"),
					resource.TestCheckResourceAttr("unionai_cluster.test", "name", "prod"),
					resource.TestCheckResourceAttr("unionai_cluster.test", "cluster_pool_name", "default"),
					resource.TestCheckResourceAttr("unionai_cluster.test", "state", "active"),
					func(*terraform.State) error {
						if record := fake.record("acme/prod"); record == nil || record.pool != "default" {
							return fmt.Errorf("unexpected cluster on server: %+v", record)
						}
						return nil
					},
				),
			},
			{
				Config:   clusterConfig("prod", "default"),
				PlanOnly: true,
			},
			// A pool move is refused while the co-named queue is still active.
			{
				Config:      clusterConfig("prod", "gpu"),
				ExpectError: regexp.MustCompile(`Unable to move cluster to another pool`),
			},
			{
				PreConfig: func() {
					fake.mu.Lock()
					fake.clusters["acme/prod"].queueDrained = true
					fake.mu.Unlock()
				},
				Config: clusterConfig("prod", "gpu"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("unionai_cluster.test", "cluster_pool_name", "gpu"),
					func(*terraform.State) error {
						if record := fake.record("acme/prod"); record.pool != "gpu" {
							return fmt.Errorf("pool on server = %q, want gpu", record.pool)
						}
						return nil
					},
				),
			},
			{
				ResourceName:      "unionai_cluster.test",
				ImportState:       true,
				ImportStateId:     "acme/prod",
				ImportStateVerify: true,
			},
			{
				ResourceName:      "unionai_cluster.test",
				ImportState:       true,
				ImportStateId:     "prod",
				ImportStateVerify: true,
			},
		},
	})
}

// A cluster registered outside Terraform is reported with import guidance, not adopted.
func TestClusterResource_AlreadyExists(t *testing.T) {
	fake, factories := newClusterTestServer(t)
	fake.clusters["acme/prod"] = &fakeClusterRecord{pool: "default", state: cluster.ClusterState_CLUSTER_STATE_ACTIVE, generation: 1}

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				Config:      clusterConfig("prod", ""),
				ExpectError: regexp.MustCompile(`terraform import unionai_cluster.example prod`),
			},
			// A queue holding the name blocks the cluster too.
			{
				Config:      clusterConfig("batch", ""),
				ExpectError: regexp.MustCompile(`Cluster already exists`),
			},
		},
	})
}

// A cluster deleted outside Terraform drops out of state and is planned for re-creation.
func TestClusterResource_DeletedOutsideTerraform(t *testing.T) {
	fake, factories := newClusterTestServer(t)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				Config: clusterConfig("prod", ""),
			},
			{
				PreConfig: func() {
					fake.mu.Lock()
					fake.clusters["acme/prod"].state = cluster.ClusterState_CLUSTER_STATE_DELETED
					fake.mu.Unlock()
				},
				Config:             clusterConfig("prod", ""),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

// Destroying a drained cluster deletes it outright, sending the generation it read.
func TestClusterResource_DestroyDrained(t *testing.T) {
	fake, factories := newClusterTestServer(t)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		CheckDestroy: func(*terraform.State) error {
			if record := fake.record("acme/prod"); record.state != cluster.ClusterState_CLUSTER_STATE_DELETED {
				return fmt.Errorf("state after destroy = %v, want DELETED", record.state)
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: clusterConfig("prod", ""),
			},
			{
				PreConfig: func() {
					fake.mu.Lock()
					record := fake.clusters["acme/prod"]
					record.state = cluster.ClusterState_CLUSTER_STATE_DRAINED
					record.generation = 3
					fake.mu.Unlock()
				},
				Config: clusterConfig("prod", ""),
				Check:  resource.TestCheckResourceAttr("unionai_cluster.test", "state", "drained"),
			},
		},
	})
}

func TestClusterResource_ReservedName(t *testing.T) {
	_, factories := newClusterTestServer(t)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				Config:      clusterConfig("default", ""),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`Reserved cluster name`),
			},
		},
	})
}

func TestClusterResource_RespectsAllowedOrgs(t *testing.T) {
	fake, factories := newClusterTestServer(t, "acme")

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "unionai_cluster" "test" {
  org  = "other"
  name = "prod"
}
`,
				ExpectError: regexp.MustCompile(`Union.ai org other is not allowed`),
			},
			{
				Config:        `resource "unionai_cluster" "test" {}`,
				ResourceName:  "unionai_cluster.test",
				ImportState:   true,
				ImportStateId: "other/prod",
				ExpectError:   regexp.MustCompile(`Union.ai org other is not allowed`),
			},
		},
	})

	if record := fake.record("other/prod"); record != nil {
		t.Errorf("other/prod was created despite allowed_orgs: %+v", record)
	}
}
