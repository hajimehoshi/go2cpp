// SPDX-License-Identifier: Apache-2.0

#include "autogen/go.h"

int main() {
  go2cpp_autogen::Go go;
  go.Bind("CallTwice",
          [](std::vector<go2cpp_autogen::BindingValue> args) -> go2cpp_autogen::BindingValue {
            args[0].Invoke();
            args[0].Invoke();
            return go2cpp_autogen::BindingValue{};
          });
  return go.Run();
}
