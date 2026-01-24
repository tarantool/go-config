package collectors_test

import (
	"context"
	"log"
	"os"
	"strings"
	"testing"

	"github.com/shoenig/test"
	"github.com/shoenig/test/must"

	"github.com/tarantool/go-config"
	"github.com/tarantool/go-config/collectors"
)

const configYaml = `
credentials:
  users:
    client:
      password: 'secret'
      roles: [super]
    peer:
      password: peering
      roles: ['replication']
      privileges:
      - permissions: ['execute']
        lua_call: ['box.info']
    storage:
      password: 'secret'
      roles: [sharding]
    tcf-replicator:
      password: SuPPerSECreT_PAssw0rd
      roles: [super]
    tcf-dml:
      password: SuPPerSECreT_PAssw0rd2
      roles: [super, puper, paratrooper]

iproto:
  advertise:
    peer:
      login: peer
    sharding:
      login: storage

sharding:
  bucket_count: 3000

roles_cfg:
  roles.tcf-worker:
    cluster_1: cluster1
    cluster_2: cluster2
    initial_status: active

    dml_users: [ tcf-dml ]

    replication_user: tcf-replicator
    replication_password: SuPPerSECreT_PAssw0rd

    status_ttl: 4
    enable_system_check: true

  roles.tcf-coordinator:
    failover_timeout: 20
    health_check_delay: 2
    max_suspect_counts: 3

  roles.metrics-export:
    http:
      - endpoints:
          - format: prometheus
            path: /metrics
        server: default

storage:
  provider: etcd
  etcd:
    prefix: /tcm
    endpoints: &etcd_ends
      - http://localhost:2379

initial-settings:
  clusters:
    - name: default-cluster
      id: 00000000-0000-0000-0000-000000000000
      storage-connection:
        provider: etcd
        etcd-connection:
          prefix: /single
          endpoints: *etcd_ends
`

func TestNewYaml(t *testing.T) {
	t.Parallel()

	fc := collectors.NewYamlCollector(os.Stdin)
	must.NotNil(t, fc)
	test.Eq(t, "yaml", fc.Name())
	test.Eq(t, config.UnknownSource, fc.Source())
	test.Eq(t, "", fc.Revision())
	test.True(t, fc.KeepOrder())
}

func TestYaml_WithName(t *testing.T) {
	t.Parallel()

	fc := collectors.NewYamlCollector(os.Stdin).WithName("custom")
	test.Eq(t, "custom", fc.Name())
}

func TestYaml_WithSourceType(t *testing.T) {
	t.Parallel()

	fc := collectors.NewYamlCollector(os.Stdin).WithSourceType(config.FileSource)
	test.Eq(t, config.FileSource, fc.Source())
}

func TestYaml_WithRevision(t *testing.T) {
	t.Parallel()

	fc := collectors.NewYamlCollector(os.Stdin).WithRevision("v1.0.0")
	test.Eq(t, "v1.0.0", fc.Revision())
}

func TestYaml_WithKeepOrder(t *testing.T) {
	t.Parallel()

	fc := collectors.NewYamlCollector(os.Stdin).WithKeepOrder(false)
	test.False(t, fc.KeepOrder())
}

func TestYaml_Read_Basic(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	reader := strings.NewReader(configYaml)

	fc := collectors.NewYamlCollector(reader)
	must.NotNil(t, fc)

	ch := fc.Read(ctx)

	values := make([]config.Value, 0, 512)
	for val := range ch {
		values = append(values, val)
	}

	// Verify values can be extracted.
	var length int

	for _, val := range values {
		var dest any

		err := val.Get(&dest)
		must.NoError(t, err)

		// Debug print.
		log.Println(val.Meta().Key, dest)

		length++
	}

	must.Eq(t, length, 39)
	must.Len(t, length, values)
}
