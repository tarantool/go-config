package config //nolint:testpackage // exercises the unexported mergeTreeInto helper

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tarantool/go-config/tree"
)

const (
	srcFirst  = "first"
	srcSecond = "second"
)

// TestMergeTreeInto_MapRecurse verifies that merging a src tree whose child is a map
// recurses into the dst counterpart rather than replacing it.
func TestMergeTreeInto_MapRecurse(t *testing.T) {
	t.Parallel()

	// dst: { server: { port: 8080, host: "localhost" } }
	dst := tree.New()
	server := tree.New()
	port := tree.New()

	port.Value = 8080
	port.Source = srcFirst

	host := tree.New()

	host.Value = "localhost"
	host.Source = srcFirst

	server.SetChild("port", port)
	server.SetChild("host", host)
	dst.SetChild("server", server)

	// src: { server: { port: 9090, ssl: true } }
	src := tree.New()
	srcServer := tree.New()
	srcPort := tree.New()

	srcPort.Value = 9090
	srcPort.Source = srcSecond

	srcSSL := tree.New()

	srcSSL.Value = true
	srcSSL.Source = srcSecond

	srcServer.SetChild("port", srcPort)
	srcServer.SetChild("ssl", srcSSL)
	src.SetChild("server", srcServer)

	mergeTreeInto(dst, src)

	serverNode := dst.Child("server")
	require.NotNil(t, serverNode)
	// Three keys: port, host, ssl.
	assert.Len(t, serverNode.ChildrenKeys(), 3)

	portNode := serverNode.Child("port")
	require.NotNil(t, portNode)
	assert.Equal(t, 9090, portNode.Value)
	assert.Equal(t, srcSecond, portNode.Source)

	hostNode := serverNode.Child("host")
	require.NotNil(t, hostNode)
	assert.Equal(t, "localhost", hostNode.Value)
	assert.Equal(t, srcFirst, hostNode.Source)

	sslNode := serverNode.Child("ssl")
	require.NotNil(t, sslNode)
	assert.Equal(t, true, sslNode.Value)
	assert.Equal(t, srcSecond, sslNode.Source)
}

// TestMergeTreeInto_LeafReplace verifies that a src leaf replaces a dst leaf.
func TestMergeTreeInto_LeafReplace(t *testing.T) {
	t.Parallel()

	dst := tree.New()
	dstLeaf := tree.New()

	dstLeaf.Value = "old"
	dstLeaf.Source = srcFirst
	dstLeaf.Revision = "1"
	dst.SetChild("key", dstLeaf)

	src := tree.New()
	srcLeaf := tree.New()

	srcLeaf.Value = "new"
	srcLeaf.Source = srcSecond
	srcLeaf.Revision = "2"
	src.SetChild("key", srcLeaf)

	mergeTreeInto(dst, src)

	keyNode := dst.Child("key")
	require.NotNil(t, keyNode)
	assert.Equal(t, "new", keyNode.Value)
	assert.Equal(t, srcSecond, keyNode.Source)
	assert.Equal(t, "2", keyNode.Revision)
}

// TestMergeTreeInto_ArrayCarry verifies that isArray is propagated from src to dst.
func TestMergeTreeInto_ArrayCarry(t *testing.T) {
	t.Parallel()

	dst := tree.New()

	src := tree.New()
	srcArr := tree.New()
	srcArr.MarkArray()

	srcArr.Source = srcFirst

	el0 := tree.New()

	el0.Value = "a"
	el0.Source = srcFirst
	srcArr.SetChild("0", el0)
	src.SetChild("roles", srcArr)

	mergeTreeInto(dst, src)

	rolesNode := dst.Child("roles")
	require.NotNil(t, rolesNode)
	assert.True(t, rolesNode.IsArray(), "isArray must be carried from src")
	assert.Equal(t, srcFirst, rolesNode.Source)

	el := rolesNode.Child("0")
	require.NotNil(t, el)
	assert.Equal(t, "a", el.Value)
}

// TestMergeTreeInto_OrderCarry verifies that when src.OrderSet() == true and
// dst.OrderSet() == false, the dst children are reordered to match src and
// dst.OrderSet() becomes true.
func TestMergeTreeInto_OrderCarry(t *testing.T) {
	t.Parallel()

	// dst has keys inserted in order: c, a, b.
	dst := tree.New()
	dstMap := tree.New()
	dstC := tree.New()

	dstC.Value = "C"

	dstA := tree.New()

	dstA.Value = "A"

	dstB := tree.New()

	dstB.Value = "B"

	dstMap.SetChild("c", dstC)
	dstMap.SetChild("a", dstA)
	dstMap.SetChild("b", dstB)
	// dst order NOT set.
	dst.SetChild("map", dstMap)

	// src has the same keys but declares order: a, b, c and marks OrderSet.
	src := tree.New()
	srcMap := tree.New()
	srcA := tree.New()

	srcA.Value = "A2"
	srcA.Source = srcSecond

	srcB := tree.New()

	srcB.Value = "B2"
	srcB.Source = srcSecond

	srcC := tree.New()

	srcC.Value = "C2"
	srcC.Source = srcSecond

	srcMap.SetChild("a", srcA)
	srcMap.SetChild("b", srcB)
	srcMap.SetChild("c", srcC)
	srcMap.SetOrderSet(true)
	src.SetChild("map", srcMap)

	mergeTreeInto(dst, src)

	mapNode := dst.Child("map")
	require.NotNil(t, mapNode)
	assert.True(t, mapNode.OrderSet(), "dst should inherit OrderSet from src")

	keys := mapNode.ChildrenKeys()
	assert.Equal(t, []string{"a", "b", "c"}, keys, "keys should be reordered to src order")
}

// TestMergeTreeInto_AnnotationCarry verifies that annotation is cloned from src
// children into dst.
func TestMergeTreeInto_AnnotationCarry(t *testing.T) {
	t.Parallel()

	dst := tree.New()

	src := tree.New()
	srcLeaf := tree.New()

	srcLeaf.Value = 42
	srcLeaf.Source = srcFirst
	srcLeaf.SetAnnotation("yaml-annotation")
	src.SetChild("key", srcLeaf)

	mergeTreeInto(dst, src)

	keyNode := dst.Child("key")
	require.NotNil(t, keyNode)
	assert.Equal(t, "yaml-annotation", keyNode.Annotation())
}
