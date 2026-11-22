package gl

import (
	"reflect"
	"strings"
	"testing"
)

// functions returns every function of the API, by its name.
func functions() map[string]reflect.StructField {
	found := make(map[string]reflect.StructField)
	for _, version := range []reflect.Type{reflect.TypeFor[V1](), reflect.TypeFor[V2](), reflect.TypeFor[V3](), reflect.TypeFor[V4]()} {
		for i := range version.NumField() {
			if field := version.Field(i); field.Type.Kind() == reflect.Func {
				found[field.Name] = field
			}
		}
	}
	return found
}

// tagTypes returns the C type of each parameter in a call tag, and of the result.
func tagTypes(t *testing.T, name, tag string) (params []string, result string) {
	t.Helper()
	symbol, ctype, ok := strings.Cut(tag, " func(")
	if !ok || symbol != "gl"+name {
		t.Fatalf("%s: the call tag %q does not link it to gl%s as a function", name, tag, name)
	}
	depth, start := 0, 0
	for i, r := range ctype {
		switch {
		case r == '(':
			depth++
		case r == ')' && depth > 0:
			depth--
		case r == ')':
			if i > start {
				params = append(params, ctype[start:i])
			}
			return params, ctype[i+1:]
		case r == ',' && depth == 0:
			params = append(params, ctype[start:i])
			start = i + 1
		}
	}
	t.Fatalf("%s: the call tag %q does not end its parameters", name, tag)
	return nil, ""
}

func isAddressInC(ctype string) bool {
	return strings.HasPrefix(ctype, "&") || strings.HasPrefix(ctype, "~")
}

// Each function is linked by its call tag, which is the symbol and its type in
// C: a pointer there has to be an address in Go, and a value must not be one.
// The functions once had a tag that no linker reads, and every 'const T *' was
// declared as the value that it points to.
func TestEveryFunctionMatchesItsCallTag(t *testing.T) {
	all := functions()
	if len(all) < 1000 {
		t.Fatalf("the API has %d functions, where OpenGL 4.6 has more than a thousand", len(all))
	}
	check := func(name, what string, goType reflect.Type, ctype string) {
		address := false
		switch goType.Kind() {
		case reflect.Pointer, reflect.UnsafePointer, reflect.String, reflect.Func:
			address = true
		case reflect.Uintptr:
			address = isAddressInC(ctype) // a handle, or a size as wide as a pointer.
		}
		if address != isAddressInC(ctype) {
			t.Errorf("%s: %s is %s in Go and '%s' in C", name, what, goType, ctype)
		}
	}
	for name, field := range all {
		params, result := tagTypes(t, name, field.Tag.Get("call"))
		if len(params) != field.Type.NumIn() {
			t.Errorf("%s: %d Go parameters for the %d in its call tag", name, field.Type.NumIn(), len(params))
			continue
		}
		for i, ctype := range params {
			check(name, "parameter "+field.Type.In(i).String(), field.Type.In(i), ctype)
		}
		if (result != "") != (field.Type.NumOut() == 1) {
			t.Errorf("%s: %d Go results for the result '%s' in its call tag", name, field.Type.NumOut(), result)
			continue
		}
		if result != "" {
			check(name, "the result", field.Type.Out(0), result)
		}
	}
}

// The declarations of the OpenGL registry that these are generated from are
// given with each of them.
func TestPointersOfKnownFunctions(t *testing.T) {
	all := functions()
	for name, want := range map[string]reflect.Type{
		// void glGenBuffers(GLsizei n, GLuint *buffers)
		"GenBuffers": reflect.TypeFor[func(int, *uint32)](),
		// void glDeleteBuffers(GLsizei n, const GLuint *buffers)
		"DeleteBuffers": reflect.TypeFor[func(int, *uint32)](),
		// const GLubyte *glGetString(GLenum name)
		"GetString": reflect.TypeFor[func(StringName) string](),
		// void *glMapBuffer(GLenum target, GLenum access)
		"MapBuffer": reflect.TypeFor[func(BufferTargetARB, BufferAccessARB) Pointer](),
		// void glTexParameterfv(GLenum target, GLenum pname, const GLfloat *params)
		"TexParameterfv": reflect.TypeFor[func(TextureTarget, TextureParameterName, *float32)](),
		// GLint glGetUniformLocation(GLuint program, const GLchar *name)
		"GetUniformLocation": reflect.TypeFor[func(uint32, string) int32](),
		// void glShaderSource(GLuint shader, GLsizei count, const GLchar *const*string, const GLint *length)
		"ShaderSource": reflect.TypeFor[func(uint32, int, **byte, *int32)](),
		// void glGetShaderInfoLog(GLuint shader, GLsizei bufSize, GLsizei *length, GLchar *infoLog)
		"GetShaderInfoLog": reflect.TypeFor[func(uint32, int, *int32, *byte)](),
		// void glGetPointerv(GLenum pname, void **params)
		"GetPointerv": reflect.TypeFor[func(GetPointervPName, *Pointer)](),
		// void glDebugMessageCallback(GLDEBUGPROC callback, const void *userParam)
		"DebugMessageCallback": reflect.TypeFor[func(DebugFunc, Pointer)](),
		// GLsync glFenceSync(GLenum condition, GLbitfield flags)
		"FenceSync": reflect.TypeFor[func(SyncCondition, SyncBehaviorFlags) uintptr](),
		// void glBufferData(GLenum target, GLsizeiptr size, const void *data, GLenum usage)
		"BufferData": reflect.TypeFor[func(BufferTargetARB, uintptr, Pointer, BufferUsageARB)](),
	} {
		field, ok := all[name]
		if !ok {
			t.Errorf("there is no %s", name)
			continue
		}
		if field.Type != want {
			t.Errorf("%s is %s, want %s", name, field.Type, want)
		}
	}
	// a vector form is named for the pointer that it takes. These four are named as one,
	// and write to a buffer object at an offset: GLintptr offset is the last parameter of each.
	inBuffer := map[string]bool{
		"GetQueryBufferObjecti64v": true, "GetQueryBufferObjectiv": true,
		"GetQueryBufferObjectui64v": true, "GetQueryBufferObjectuiv": true,
	}
	for name, field := range all {
		if !strings.HasSuffix(name, "v") || inBuffer[name] {
			continue
		}
		pointer := false
		for i := range field.Type.NumIn() {
			kind := field.Type.In(i).Kind()
			pointer = pointer || kind == reflect.Pointer || kind == reflect.UnsafePointer || kind == reflect.String
		}
		if !pointer {
			t.Errorf("%s is a vector form with no pointer among its parameters: %s", name, field.Type)
		}
	}
}
