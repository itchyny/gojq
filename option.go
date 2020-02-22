package gojq

// CompilerOption ...
type CompilerOption func(*compiler)

// WithModulePaths is a compiler option for module paths.
func WithModulePaths(modulePaths []string) CompilerOption {
	return func(c *compiler) {
		c.modulePaths = modulePaths
	}
}

// WithVariables is a compiler option for variable names.
func WithVariables(variables []string) CompilerOption {
	return func(c *compiler) {
		c.variables = variables
	}
}
