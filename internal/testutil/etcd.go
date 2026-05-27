package testutil

import (
	"context"
	"testing"
	"time"

	"github.com/tarantool/go-storage"
	etcddriver "github.com/tarantool/go-storage/driver/etcd"
	etcdtest "github.com/tarantool/go-storage/test_helpers/etcd"
	etcdclient "go.etcd.io/etcd/client/v3"
)

const (
	etcdDialTimeout  = 5 * time.Second
	etcdResetTimeout = 5 * time.Second
)

// EtcdTestCluster holds the resources for an embedded etcd test cluster.
type EtcdTestCluster struct {
	Storage storage.Storage
	Client  *etcdclient.Client
}

// sharedEtcd is the LazyCluster injected by TestMain via SetSharedEtcd.
// Restarting embed per test is expensive and exposes embed's process-global
// state (Prometheus registry, logger builder) — back-to-back starts have
// been observed to wedge raft and burn the suite timeout. Sharing one
// cluster sidesteps that; each test wipes the keyspace and gets a fresh
// client. The integration package runs tests sequentially (see its package
// doc), so a shared cluster is safe.
var sharedEtcd *etcdtest.LazyCluster //nolint:gochecknoglobals

// SetSharedEtcd installs the process-wide embedded etcd used by
// NewEtcdTestCluster. Call from TestMain before m.Run().
func SetSharedEtcd(c *etcdtest.LazyCluster) {
	sharedEtcd = c
}

// NewEtcdTestCluster returns a storage.Storage and an etcd client backed by
// the process-wide embedded etcd installed via SetSharedEtcd. The keyspace
// is wiped before returning so each test sees an empty store. Tests using
// this helper are skipped in short mode.
func NewEtcdTestCluster(t *testing.T) *EtcdTestCluster {
	t.Helper()

	if testing.Short() {
		t.Skip("skipping integration tests in short mode")
	}

	if sharedEtcd == nil {
		t.Fatal("testutil: shared etcd not initialised; call SetSharedEtcd from TestMain")
	}

	client, err := etcdclient.New(etcdclient.Config{ //nolint:exhaustruct
		Endpoints:   sharedEtcd.EndpointsGRPC(),
		DialTimeout: etcdDialTimeout,
	})
	if err != nil {
		t.Fatalf("Failed to create etcd client: %v", err)
	}

	t.Cleanup(func() { _ = client.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), etcdResetTimeout)
	defer cancel()

	_, err = client.Delete(ctx, "\x00", etcdclient.WithFromKey())
	if err != nil {
		t.Fatalf("Failed to clear etcd keyspace: %v", err)
	}

	driver := etcddriver.New(client)

	return &EtcdTestCluster{
		Storage: storage.NewStorage(driver),
		Client:  client,
	}
}
