package tarantool

import config "github.com/tarantool/go-config/v2"

// TemplateVariables returns Tarantool's built-in template variables for a path
// of the form groups/<group>/replicasets/<replicaset>/instances/<instance>.
// Each valid call returns a fresh map. Other path shapes return nil.
func TemplateVariables(path config.KeyPath) map[string]string {
	const instancePathLength = 6
	if len(path) != instancePathLength || path[0] != "groups" || path[2] != "replicasets" || path[4] != "instances" {
		return nil
	}

	return map[string]string{"group_name": path[1], "replicaset_name": path[3], "instance_name": path[5]}
}
