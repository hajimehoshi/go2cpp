// SPDX-License-Identifier: Apache-2.0

#include "autogen/game.h"

#include "glfwdriver.h"

int main() {
  go2cpp_autogen::Game game(std::make_unique<GLFWDriver>());
  return game.Run();
}
