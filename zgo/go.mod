module runtime.link/zgo

go 1.24.0

toolchain go1.24.2

replace runtime.link => ../

require (
	golang.org/x/tools v0.34.0
	runtime.link v0.0.0-00010101000000-000000000000
)

require (
	golang.org/x/mod v0.25.0 // indirect
	golang.org/x/sync v0.15.0 // indirect
)
