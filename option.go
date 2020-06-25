package gojq

// CompilerOption ...
type CompilerOption func(*compiler)

// WithModuleLoader is a compiler option for module loader.
// If you want to load modules from the filesystem, use NewModuleLoader.
func WithModuleLoader(moduleLoader ModuleLoader) CompilerOption {
	return func(c *compiler) {
		c.moduleLoader = moduleLoader
	}
}

// WithEnvironLoader is a compiler option for environment variables loader.
// The OS environment variables are not accessible by default due to security
// reason. You can pass os.Environ if you allow to access it.
func WithEnvironLoader(environLoader func() []string) CompilerOption {
	return func(c *compiler) {
		c.environLoader = environLoader
	}
}

// WithVariables is a compiler option for variable names. The variables can be
// used in the query. You have to give the values to query.Run or code.Run in
// the same order.
func WithVariables(variables []string) CompilerOption {
	return func(c *compiler) {
		c.variables = variables
	}
}

// WithInputIter is a compiler option for input iterator used by input(s)/0.
// Note that input and inputs functions are not allowed by default. We have
// to distinguish the query input and the values for input(s) functions. For
// example, consider using inputs with --null-input. If you want to allow
// input(s) functions, create an Iter and use WithInputIter option.
func WithInputIter(inputIter Iter) CompilerOption {
	return func(c *compiler) {
		c.inputIter = inputIter
	}
}
