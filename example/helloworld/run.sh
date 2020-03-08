env GOOS=js GOARCH=wasm go build -tags example -o helloworld.wasm .
go run ../../ -wasm helloworld.wasm -namespace Go2DotNet.Example.HelloWorld.AutoGen > go.cs
dotnet run .
