set -e
env GOOS=js GOARCH=wasm go test -c -o binding.wasm -trimpath .
rm -rf autogen
go run ../../cmd/gowasm2cpp -out autogen -wasm binding.wasm -namespace Go2DotNet.Test.Binding.AutoGen
dotnet run . -- -test.v
