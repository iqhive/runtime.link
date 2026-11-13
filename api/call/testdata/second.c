// A second library for the tests of the order in which libraries are searched,
// with one symbol that helpers.c has under the same name and one that it lacks.
int only_in_second(void) { return 77; }
int compiled_by_clang(void) { return 55; }
