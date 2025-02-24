# Contributing to runtime.link

Thank you for your interest in contributing to runtime.link! This document provides guidelines and setup instructions for contributing to the project.

## Project Philosophy

runtime.link aims to be dependency-free and follows specific design values:

1. Use full readable words for identifiers rather than abbreviations (e.g., `PutString` over `puts`)
2. Use acronyms as package names and/or as a suffix (e.g., `TheExampleAPI` over `TheAPIExample`)
3. Use explicitly tagged types that define data relationships rather than implicit primitives
4. Don't stutter exported identifiers (e.g., `customer.Account` over `customer.Customer`)

## Development Setup

1. Clone the repository:
```bash
gh repo clone iqhive/runtime.link
cd runtime.link
```

2. Install Go (version 1.21 or later recommended)

3. Run tests:
```bash
go test ./...
```

## Linter Configuration

runtime.link adopts a different convention for Go struct tags, which permits multi-line and inline-documentation. Configure your development environment as follows:

### govet
```bash
go vet -structtag=false ./...
```

### VS Code + gopls
```json
{
    "go.vetFlags": [
        "-structtag=false"
    ],
    "gopls": {
        "analyses": {
            "structtag": false
        }
    }
}
```

### Zed
```json
{
    "lsp": {
        "gopls": {
            "initialization_options": {
                "analyses": {
                    "structtag": false
                }
            }
        }
    }
}
```

### golangci-lint.yml
```yaml
linters-settings:
  govet:
    disable:
      - structtag # support runtime.link convention.
```

## Documentation Standards

1. Package Documentation
   - Each package should have a package-level comment describing its purpose
   - Use full sentences and proper punctuation
   - Include examples where appropriate

2. Struct Tags
   - Can be multi-line
   - First line contains the tags
   - Subsequent lines contain documentation
   - Documentation lines should be properly indented

Example:
```go
type API struct {
    HelloWorld func() string `cmdl:"hello_world" link:"example_helloworld func()$char"
        returns the string "Hello World"
        this is an example of multi-line documentation`
}
```

## Pull Request Process

1. Create a new branch for your changes
2. Make your changes following the project's design values
3. Ensure documentation is updated if needed
4. Submit a pull request with a clear description of changes

## Questions or Issues?

Feel free to open a GitHub Discussion for any ideas or questions you may have. Note that we currently cannot accept pull requests for new top-level packages as we're focusing on maintaining a well-defined and cohesive design space.
