package gojq

// CompilerOption ...
type CompilerOption func(*compiler)

// WithModuleLoader is a compiler option for module loader.
func WithModuleLoader(moduleLoader ModuleLoader) CompilerOption {
	return func(c *compiler) {
		c.moduleLoader = moduleLoader
	}
}

// WithEnvironLoader is a compiler option for environment variables loader.
// The OS environment variables is not accessible by default due to security
// reason. You can pass os.Environ if you allow to access it.
func WithEnvironLoader(environLoader func() []string) CompilerOption {
	return func(c *compiler) {
		c.environLoader = environLoader
	}
}

// WithVariables is a compiler option for variable names.
func WithVariables(variables []string) CompilerOption {
	return func(c *compiler) {
		c.variables = variables
	}
}
