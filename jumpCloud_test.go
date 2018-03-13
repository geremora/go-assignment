package main

import (
	"testing"
)

func Test(t *testing.T) {

	testSHA512(t, "angryMonkey", 
		"ZEHhWB65gUlzdVwtDQArEyx+KVLzp/aTaRaPlBzYRIFj6vjFdqEb0Q5B8zVKCZ0vKbZPZklJz0Fd7su2A+gf7Q==")

}

func testSHA512(t *testing.T, str string,
	expected string) {
	result := SHA512(str)
	if result != expected {
		t.Errorf("unit test failure: "+
			"input: %s, expected: %s, result: %s",
			str, expected, result)
	}
}
