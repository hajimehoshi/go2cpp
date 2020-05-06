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

You can invoke the registered functions via `syscall/js.Global().Get("c++")`:

```go
package main

import (
	"syscall/js"
)

func main() {
	i := 0
	f := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		println("Hi", i)
		i++
		return nil
	})
	defer f.Release()
	js.Global().Get("c++").Call("CallTwice", f)
}
```

## TODO

  * Improving compiling speed by reducing C++ files
  * `net/http`
  * `os`
