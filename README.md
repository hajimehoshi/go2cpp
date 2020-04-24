# go2dotnet

A converter from Go to .NET (C#)

## Example

```sh
cd example/helloworld
./run.sh
```

## How does this work?

This tool analyses a Wasm file compiled from Go files, and generates C# files based on the Wasm file.

## Calling C# code from Go

You can use `syscall/js`. You can access C# world via `js.Global().Get(".net").Get("Something.That.Can.Be.Reached.By.Reflection")`.

See `example/binding` for actual usages.

## TODO

  * Improving compiling speed by reducing C# files
  * `net/http`
  * `os`
