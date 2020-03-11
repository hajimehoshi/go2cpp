for pkg in fmt testing; do
  echo "# Test $pkg"
  env GOOS=js GOARCH=wasm go test -c -o test.wasm -trimpath $pkg
  go run .. -wasm test.wasm -namespace Go2DotNet.Test.AutoGen > gen.cs
  dotnet run .
done
