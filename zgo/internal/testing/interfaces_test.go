package main

import "testing"

type Speaker interface {
	Speak() string
}

type Dog struct{}

func (d Dog) Speak() string {
	return "Woof!"
}

func TestInterfaces(t *testing.T) {
	var s Speaker = Dog{}
	if s.Speak() != "Woof!" {
		t.FailNow()
	}
}

type Animal struct {
	Name string
}

type Cat struct {
	Animal
}

func TestEmbedding(t *testing.T) {
	c := Cat{Animal{Name: "Whiskers"}}
	if c.Name != "Whiskers" {
		t.FailNow()
	}
}
