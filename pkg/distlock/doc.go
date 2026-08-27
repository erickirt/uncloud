// Package distlock provides distributed, automatically renewed leases across independent nodes.
//
// A Cluster must return every node in the lock group, including temporarily unavailable nodes, because every node
// counts toward quorum. Changing the node set is unsafe if a new quorum can be disjoint from an earlier quorum while
// leases acquired from the earlier node set may still be valid. Callers must stop protected work when the context
// returned by Lease.Context is done.
package distlock
