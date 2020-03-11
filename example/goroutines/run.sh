env GOOS=js GOARCH=wasm go build -tags example -o goroutines.wasm -trimpath .
go run ../../ -wasm goroutines.wasm -namespace Go2DotNet.Example.Goroutines.AutoGen > gen.cs
dotnet run .
