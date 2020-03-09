env GOOS=js GOARCH=wasm go build -tags example -o helloworld.wasm -trimpath .
go run ../../ -wasm helloworld.wasm -namespace Go2DotNet.Example.HelloWorld.AutoGen > gen.cs
dotnet run .
