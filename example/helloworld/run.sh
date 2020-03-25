set -e
env GOOS=js GOARCH=wasm go build -tags example -o helloworld.wasm -trimpath .
go run ../../cmd/gowasm2csharp -wasm helloworld.wasm -namespace Go2DotNet.Example.HelloWorld.AutoGen
dotnet run .
