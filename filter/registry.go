package filter

import (
	"fmt"
	"log"
)

func init() {
	// TODO: Register filters
}

// --- actual registry code below here ---

var filters = make(map[string]CanBeInstanced)

func findByName(name string) (CanBeInstanced, error) {
	f, exists := filters[name]
	if !exists {
		return nil, fmt.Errorf("filter %s not found", name)
	}
	return f, nil
}

func mustRegister(name string, f CanBeInstanced) {
	if _, exists := filters[name]; exists {
		panic(fmt.Errorf("filter already registered with name %s", name))
	}
	filters[name] = f
	log.Printf("Registered %s as %#v", name, f)
}
