// Package assets embeds editable text resources into the binary.
package assets

import _ "embed"

// FinalMessage is shown in the modal after the last achievement is unlocked.
// Edit final_message.txt and rebuild, or drop a final_message.txt next to the
// state file to override it without rebuilding.
//
//go:embed final_message.txt
var FinalMessage string
