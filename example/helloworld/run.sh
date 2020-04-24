set -e
env GOOS=js GOARCH=wasm go build -tags example -o helloworld.wasm -trimpath .
rm -rf autogen
go run ../../cmd/gowasm2csharp -out autogen -wasm helloworld.wasm -namespace Go2DotNet.Example.HelloWorld.AutoGen
dotnet run .
