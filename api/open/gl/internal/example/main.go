package main

import (
	"unsafe"

	"runtime.link/api/open/gl"
)

const vertexShaderSource = `#version 330 core
    layout(location = 0) in vec3 aPos;
    void main() {
        gl_Position = vec4(aPos, 1.0);
    }`

// Fragment Shader source code
const fragmentShaderSource = `#version 330 core
    out vec4 FragColor;
    void main() {
        FragColor = vec4(1.0, 0.5, 0.2, 1.0);
    }`

func main() {
	var GL gl.API

	GL.Viewport(0, 0, 800, 600)
	var vertices = []float32{
		-0.5, -0.5, 0.0, // Bottom-left
		0.5, -0.5, 0.0, // Bottom-right
		0.0, 0.5, 0.0, // Top
	}
	var VAO, VBO uint32
	GL.GenVertexArrays(1, &VAO)
	GL.GenBuffers(1, &VBO)
	GL.BindVertexArray(VAO)
	GL.BindBuffer(gl.BufferTargetARBGL_ARRAY_BUFFER, VBO)
	GL.BufferData(gl.BufferTargetARBGL_ARRAY_BUFFER, unsafe.Sizeof(vertices[0])*uintptr(len(vertices)), gl.Pointer(&vertices[0]), gl.BufferUsageARBGL_STATIC_DRAW)

	GL.VertexAttribPointer(0, 3, gl.VertexAttribPointerTypeGL_FLOAT, false, int(3*unsafe.Sizeof(float32(0))), nil)
	GL.EnableVertexAttribArray(0)

	var vertexShader = GL.CreateShader(gl.ShaderTypeGL_VERTEX_SHADER)
	GL.ShaderSource(vertexShader, 1, vertexShaderSource[0], 0)
	GL.CompileShader(vertexShader)

	var success int32
	GL.GetShaderiv(vertexShader, gl.ShaderParameterNameGL_COMPILE_STATUS, &success)
	if success == 0 {
		var infoLog [512]byte
		GL.GetShaderInfoLog(vertexShader, 512, nil, &infoLog[0])
		panic("ERROR::SHADER::VERTEX::COMPILATION_FAILED\n" + string(infoLog[:]))
	}

	var fragmentShader = GL.CreateShader(gl.ShaderTypeGL_FRAGMENT_SHADER)
	GL.ShaderSource(fragmentShader, 1, fragmentShaderSource[0], 0)
	GL.CompileShader(fragmentShader)

	GL.GetShaderiv(fragmentShader, gl.ShaderParameterNameGL_COMPILE_STATUS, &success)
	if success == 0 {
		var infoLog [512]byte
		GL.GetShaderInfoLog(fragmentShader, 512, nil, &infoLog[0])
		panic("ERROR::SHADER::FRAGMENT::COMPILATION_FAILED\n" + string(infoLog[:]))
	}

	var shaderProgram = GL.CreateProgram()
	GL.AttachShader(shaderProgram, vertexShader)
	GL.AttachShader(shaderProgram, fragmentShader)
	GL.LinkProgram(shaderProgram)

	GL.GetProgramiv(shaderProgram, gl.ProgramPropertyARBGL_LINK_STATUS, &success)
	if success == 0 {
		var infoLog [512]byte
		GL.GetProgramInfoLog(shaderProgram, 512, nil, &infoLog[0])
		panic("ERROR::SHADER::PROGRAM::LINKING_FAILED\n" + string(infoLog[:]))
	}

	GL.DeleteShader(vertexShader)
	GL.DeleteShader(fragmentShader)

	for {
		GL.ClearColor(0.2, 0.3, 0.3, 1.0)
		GL.Clear(gl.ClearBufferMaskGL_COLOR_BUFFER_BIT)

		GL.UseProgram(shaderProgram)
		GL.BindVertexArray(VAO)
		GL.DrawArrays(gl.PrimitiveTypeGL_TRIANGLES, 0, 3)

		// Swap buffers and poll events (not implemented here)
		// glfw.SwapBuffers(window)
		// glfw.PollEvents()

		// Break condition for the loop (not implemented here)
		break
	}
	GL.DeleteVertexArrays(1, VAO)
	GL.DeleteBuffers(1, VBO)
	GL.DeleteProgram(shaderProgram)
}
