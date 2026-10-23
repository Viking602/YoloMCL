package distribution

import "strings"

// msClientID is the Azure application (client) ID on Mojang's allow list,
// accepted as a legacy linker override. The shared build command now places
// MS_CLIENT_ID in encoded on every platform. Builds without either keep
// Microsoft sign-in off.
var msClientID string

func embeddedMicrosoftClientID() string { return strings.TrimSpace(msClientID) }
