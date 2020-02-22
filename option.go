package gojq

// CompilerOption ...
type CompilerOption func(*compiler)

// WithVariables is a compiler option for variable names.
func WithVariables(variables []string) CompilerOption {
	return func(c *compiler) {
		c.variables = variables
	}
}
