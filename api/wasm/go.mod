module github.com/iqhive/runtime.link/api/wasm

go 1.25.0

toolchain go1.27.4

replace github.com/iqhive/runtime.link => ../../

require (
	github.com/iqhive/runtime.link v0.0.0-00010101000000-000000000000
	github.com/tetratelabs/wazero v1.9.0
)
