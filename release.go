// +build !debug

package gojq

func (c *compiler) appendCodeInfo(string) {}

func (env *env) debugCodes() {}

func (env *env) debugState(int) {}

func (env *env) debugForks(int, string) {}
