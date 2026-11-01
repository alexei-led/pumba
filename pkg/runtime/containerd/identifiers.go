package containerd

import "crypto/rand"

// newOperationID avoids collisions with concurrent Pumba processes and with
// containers, snapshots or execs left behind by a previous invocation.
func newOperationID(prefix string) string {
	return prefix + "-" + rand.Text()
}
