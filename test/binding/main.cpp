// SPDX-License-Identifier: Apache-2.0

#include "autogen/go.h"

int main(int argc, char *argv[]) {
  go2cpp_autogen::Go go;
  go.Bind("Identity",
          [](std::vector<go2cpp_autogen::BindingValue> args) -> go2cpp_autogen::BindingValue {
            return args[0];
          });
  go.Bind("Invoke",
          [](std::vector<go2cpp_autogen::BindingValue> args) -> go2cpp_autogen::BindingValue {
            return args[0].Invoke();
          });
  go.Bind("Sum",
          [](std::vector<go2cpp_autogen::BindingValue> args) -> go2cpp_autogen::BindingValue {
            double sum = 0;
            for (auto v : args) {
              sum += v.ToNumber();
            }
            return go2cpp_autogen::BindingValue{sum};
          });
  return go.Run(argc, argv);
}
