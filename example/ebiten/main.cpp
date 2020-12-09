// SPDX-License-Identifier: Apache-2.0

#include "autogen/gl.h"
#include "autogen/go.h"

#include <GLFW/glfw3.h>
#include <dlfcn.h>

int main() {
  if (!glfwInit()) {
    return EXIT_FAILURE;
  }

  glfwWindowHint(GLFW_CLIENT_API, GLFW_OPENGL_API);
  glfwWindowHint(GLFW_CONTEXT_VERSION_MAJOR, 2);
  glfwWindowHint(GLFW_CONTEXT_VERSION_MINOR, 1);

  GLFWwindow *window =
      glfwCreateWindow(640, 480, "Ebiten test", nullptr, nullptr);
  if (!window) {
    glfwTerminate();
    return EXIT_FAILURE;
  }
  glfwMakeContextCurrent(window);
  glfwSwapInterval(1);

  using Value = go2cpp_autogen::Value;

  auto& global = go2cpp_autogen::Value::Global().ToObject();
  auto go2cpp = std::make_shared<go2cpp_autogen::DictionaryValues>();
  global.Set("go2cpp", Value{go2cpp});

  auto gl = std::make_shared<go2cpp_autogen::GL>(
      [](const char *name) -> void * { return dlsym(RTLD_DEFAULT, name); });
  go2cpp->Set("gl", Value{gl});

  go2cpp->Set("screenWidth", Value{640.0});
  go2cpp->Set("screenHeight", Value{480.0});

  go2cpp_autogen::Go go;
  global.Set("requestAnimationFrame",
             Value{std::make_shared<go2cpp_autogen::Function>(
                 [&go, window](Value self, std::vector<Value> args) -> Value {
                   go.EnqueueTask([args, window]() {
                     glfwPollEvents();
                     Value f = args[0];
                     f.ToObject().Invoke(Value{}, {});
                     glfwSwapBuffers(window);
                   });
                   return Value{};
                 })});
  return go.Run();
}
