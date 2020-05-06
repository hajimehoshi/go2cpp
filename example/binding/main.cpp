// SPDX-License-Identifier: Apache-2.0

#include "autogen/go.h"

int main() {
  go2cpp_autogen::Go go;
  go.Bind("CallTwice",
          [](std::vector<go2cpp_autogen::GoValue> args) -> go2cpp_autogen::GoValue {
            args[0].Invoke();
            args[0].Invoke();
            return go2cpp_autogen::GoValue{};
          });
  return go.Run();
}
