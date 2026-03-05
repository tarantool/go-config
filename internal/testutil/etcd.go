package testutil

import (
	"testing"
	"time"

	"github.com/tarantool/go-storage"
	etcddriver "github.com/tarantool/go-storage/driver/etcd"
	"go.etcd.io/etcd/client/pkg/v3/testutil"
	etcdclient "go.etcd.io/etcd/client/v3"
	etcdfintegration "go.etcd.io/etcd/tests/v3/framework/integration"
)

const (
	etcdDialTimeout = 5 * time.Second
)

// silentTB wraps a testutil.TB and discards all logs.
type silentTB struct {
	testutil.TB
}

func (s *silentTB) Log(_ ...any)            {}
func (s *silentTB) Logf(_ string, _ ...any) {}

// EtcdTestCluster holds the resources for an embedded etcd test cluster.
type EtcdTestCluster struct {
	Storage storage.Storage
	Client  *etcdclient.Client
}

// NewEtcdTestCluster starts an embedded single-node etcd cluster and returns
// a storage.Storage backed by it along with a raw etcd client for direct access.
// The cluster is terminated and the client is closed when the test finishes.
// Tests using this helper are skipped in short mode.
func NewEtcdTestCluster(t *testing.T) *EtcdTestCluster {
	t.Helper()

	if testing.Short() {
		t.Skip("skipping integration tests in short mode")
	}

	etcdfintegration.BeforeTest(
		&silentTB{TB: t},
		etcdfintegration.WithoutGoLeakDetection(),
	)

	cluster := etcdfintegration.NewCluster(
		&silentTB{TB: t},
		&etcdfintegration.ClusterConfig{Size: 1}, //nolint:exhaustruct
	)
	t.Cleanup(func() { cluster.Terminate(nil) })

	endpoints := cluster.Client(0).Endpoints()

	client, err := etcdclient.New(etcdclient.Config{ //nolint:exhaustruct
		Endpoints:   endpoints,
		DialTimeout: etcdDialTimeout,
	})
	if err != nil {
		t.Fatalf("Failed to create etcd client: %v", err)
	}

	t.Cleanup(func() { _ = client.Close() })

	driver := etcddriver.New(client)

	return &EtcdTestCluster{
		Storage: storage.NewStorage(driver),
		Client:  client,
	}
}
