set -e
env GOOS=js GOARCH=wasm go build -tags example -o binding.wasm -trimpath .
rm -rf autogen
go run ../../cmd/gowasm2cpp -out autogen -wasm binding.wasm -namespace Go2DotNet.Example.Binding.AutoGen
dotnet run .
