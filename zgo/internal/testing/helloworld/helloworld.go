package main

func result() int {
	return 22
}

func add(a, b int) int {
	return a + b
}

func main() {
	var p = new(int)
	*p = 22
	println(*p)
	println("Hello World")
	println(result())
	println(add(1, 2))

	var s = make([]int, 0)
	s = append(s, 1)
	println(s[0])

	var c = make([]int, 1)
	copy(c, s)
	println(c[0])

	clear(c)
	println(c[0])

	var t = make(map[string]string)
	t["key"] = "value"
	println(t["key"])
}
