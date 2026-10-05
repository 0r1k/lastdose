//go:build !release

package assets

// key is nil without the release tag: the message stays sealed and state is
// not signed.
func key() []byte { return nil }
