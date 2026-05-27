package integration_test

import (
	"os"
	"testing"

	etcdtest "github.com/tarantool/go-storage/test_helpers/etcd"

	"github.com/tarantool/go-config/internal/testutil"
)

// TestMain owns the lifecycle of the embedded etcd shared across the
// integration tests. We start a single LazyCluster, hand it to the testutil
// helper, run the suite, then terminate the cluster before exit so the
// embed goroutines and temp dir are released cleanly.
func TestMain(m *testing.M) {
	os.Exit(run(m))
}

func run(m *testing.M) int {
	cluster := etcdtest.NewLazyCluster(etcdtest.ClusterConfig{Size: 1})
	defer cluster.Terminate()

	testutil.SetSharedEtcd(cluster)

	return m.Run()
}
