# go2cpp

A converter from Go to C++

## Example

```sh
cd example/helloworld
./run.sh
```

## How does this work?

This tool analyses a Wasm file compiled from Go files, and generates C++ files based on the Wasm file.

## Binding API

On C++ side, you can register C++ functions via `Go::Bind`:

```cpp
#include <autogen/go.h>

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
```

You can invoke the registered functions via `github.com/hajimehoshi/go2cpp/binding.Call`:

```go
package main

import (
	"github.com/hajimehoshi/go2cpp/binding"
)

func main() {
	i := 0
	f := binding.FuncOf(func(args []binding.Value) interface{} {
		println("Hi", i)
		i++
		return nil
	})
	defer f.Release()

	// For the definition of CallTwice, see main.cpp.
	binding.Call("CallTwice", f)
}
```

## TODO

  * Improving compiling speed by reducing C++ files
  * `net/http`
  * `os`
