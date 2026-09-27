//go:build darwin || linux

package main

func goCommandName() string  { return "go" }
func gitCommandName() string { return "git" }

func platformContractEnv(_, _, _ string) []string { return nil }
