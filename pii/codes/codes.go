package codes

import "runtime.link/pii"

type (
	// Postal code, such as a ZIP code or a postal code.
	Postal pii.String

	// Identity code, such as a social security number, license number or taxpayer identifier.
	Identity pii.String
)
