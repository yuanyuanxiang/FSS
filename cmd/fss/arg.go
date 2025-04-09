package main

// Args contains apps console arguments
type Args []string

// Args returns the command line arguments associated
func NewArgs(params []string) Args {
	return Args(params)
}

// Get returns the nth argument, or else a blank string
func (a Args) Get(n int) string {
	if len(a) > n {
		return a[n]
	}
	return ""
}

// First returns the first argument, or else a blank string
func (a Args) First() string {
	return a.Get(0)
}

// Tail returns the rest of the arguments (not the first one)
// or else an empty string slice
func (a Args) Tail() []string {
	if len(a) >= 2 {
		return []string(a)[1:]
	}
	return []string{}
}

// Present checks if there are any arguments present
func (a Args) Present() bool {
	return len(a) != 0
}
