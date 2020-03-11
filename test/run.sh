env GOOS=js GOARCH=wasm go test -c -o test.wasm -trimpath testing
go run .. -wasm test.wasm -namespace Go2DotNet.Test.AutoGen -profile > gen.cs
dotnet run -- -v
