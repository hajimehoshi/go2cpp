// SPDX-License-Identifier: Apache-2.0

#include "autogen/go.h"

int main(int argc, char *argv[]) {
  go2cpp_autogen::Go go;
  go.Bind("Identity",
          [](std::vector<go2cpp_autogen::BindingValue> args) -> go2cpp_autogen::BindingValue {
            return args[0];
          });
  return go.Run(argc, argv);
}
