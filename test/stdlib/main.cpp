// SPDX-License-Identifier: Apache-2.0

#include "autogen/go.h"

int main(int argc, char *argv[]) {
  go2cpp_autogen::Go go;
  go.Run(argc, argv);
  return 0;
}
