package gojq

// CompilerOption ...
type CompilerOption func(*compiler)

// WithModuleLoader is a compiler option for module loader.
func WithModuleLoader(moduleLoader ModuleLoader) CompilerOption {
	return func(c *compiler) {
		c.moduleLoader = moduleLoader
	}
}

// WithVariables is a compiler option for variable names.
func WithVariables(variables []string) CompilerOption {
	return func(c *compiler) {
		c.variables = variables
	}
}
