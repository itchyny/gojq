// +build !debug

package gojq

func (c *compiler) appendCodeInfo(string) {}

func (env *env) debugCodes() {}

func (env *env) debugState(int, bool) {}

func (env *env) debugForks(int, string) {}
