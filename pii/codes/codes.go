package codes

import "runtime.link/pii"

type (
	// Location code, such as a ZIP code, area code or a postal code.
	Location pii.String

	// Country code.
	Country pii.String

	// Identity code, such as a social security number, license number or taxpayer identifier.
	Identity pii.String

	// Language code, identifies a spoken or written language.
	Language pii.String

	// Customer code that identifies a customer for a particular company.
	Customer pii.String

	// Password used for authentication.
	Password pii.String
)
