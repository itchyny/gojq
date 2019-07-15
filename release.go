// +build !debug

package gojq

func (c *compiler) appendCodeInfo(interface{}) {}

func (c *compiler) deleteCodeInfo(string) {}

func (env *env) debugCodes() {}

func (env *env) debugState(int, bool) {}

func (env *env) debugForks(int, string) {}
