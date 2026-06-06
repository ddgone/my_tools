package main

func cloneStringSlice(values []string) []string {
	return append([]string{}, values...)
}
