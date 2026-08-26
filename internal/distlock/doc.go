// Package distlock provides distributed, automatically renewed leases across Uncloud machines.
//
// The initial in-memory store does not preserve leases across machine daemon restarts. Cluster membership must remain
// stable while leases are active. Callers must stop protected work when the context returned by Lease.Context is done.
package distlock
