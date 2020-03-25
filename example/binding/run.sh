set -e
env GOOS=js GOARCH=wasm go build -tags example -o binding.wasm -trimpath .
go run ../../cmd/gowasm2csharp -wasm binding.wasm -namespace Go2DotNet.Example.Binding.AutoGen
dotnet run .
