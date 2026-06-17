# Development Guidelines for Go Projects

This document outlines the development guidelines for Go projects in this repository. Following these guidelines ensures consistency, maintainability, and best practices across the codebase.

## 1. Environment Variables for Configuration

- All environment settings **must** be configured using environment variables
- No hardcoded configuration values are allowed in the code
- Environment variables should be validated at startup
- Use descriptive names for environment variables, prefixed with the application or service name
- Document all required environment variables in the README.md

Example:
```
// Good
apiKey := os.Getenv("HELLOASSO_API_KEY")
if apiKey == "" {
    slog.Error("Missing environment variable", "variable", "HELLOASSO_API_KEY")
    return fmt.Errorf("HELLOASSO_API_KEY environment variable must be set")
}

// Bad - Don't do this
apiKey := "hardcoded-api-key"
```

## 2. Logging

- Use **only** the `log/slog` package for logging
- Direct `fmt.Print*` calls should not be used for logging
- Use appropriate log levels (Debug, Info, Warn, Error)
- Include contextual information in log messages using structured logging
- Configure the logger at the application startup

Example:
```
// Good
logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
slog.SetDefault(logger)
slog.Info("Processing data", "count", len(items))

// Bad - Don't do this
fmt.Printf("Processing %d items\n", len(items))
```

## 3. External Service Calls

- All external service calls must be implemented in dedicated packages under the `services` directory
- Follow the pattern: `services/<servicename>/<servicename>.go`
- Each service package should:
  - Encapsulate all API calls to a specific external service
  - Handle authentication, retries, and error handling
  - Provide a clean API for the rest of the application
  - Use environment variables for configuration

Example:
```
services/
  └── helloasso/
      └── helloasso.go  // Contains all calls to HelloAsso API
  └── brevo/
      └── brevo.go      // Contains all calls to Brevo API
```

## 4. Main Package Structure

- The `main.go` file should contain the main application logic
- It should orchestrate the flow of the application
- Keep the `main.go` file clean and focused on high-level logic
- Delegate implementation details to appropriate packages
- Initialize logging, configuration, and services in the main function

Example:
```
func main() {
    // Initialize logging
    logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
    slog.SetDefault(logger)

    // Validate environment variables
    if os.Getenv("REQUIRED_VAR") == "" {
        slog.Error("Missing required environment variable", "variable", "REQUIRED_VAR")
        os.Exit(1)
    }

    // Call external services
    data, err := externalservice.FetchData()
    if err != nil {
        slog.Error("Failed to fetch data", "error", err)
        os.Exit(1)
    }

    // Process data
    results := processData(data)

    // Output results
    slog.Info("Processing complete", "results", len(results))
}
```

## 5. Data Processing

- Use samber/lo for data processing
- Prefer functional programming patterns for data transformations
- Use lo.Filter, lo.Map, lo.GroupBy, and other utility functions for cleaner code
- Avoid manual loops for data transformations when lo functions can be used

Example:
```
// Good - Using samber/lo
filteredData := lo.Filter(items, func(item Item, _ int) bool {
    return item.Value > 10
})

// Bad - Don't do this
var filteredData []Item
for _, item := range items {
    if item.Value > 10 {
        filteredData = append(filteredData, item)
    }
}
```

## Conclusion

Following these guidelines ensures that our codebase remains maintainable, secure, and consistent. These practices help new developers understand the codebase quickly and make it easier to extend and modify the application in the future.
