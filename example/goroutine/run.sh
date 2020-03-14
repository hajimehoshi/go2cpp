env GOOS=js GOARCH=wasm go build -tags example -o goroutine.wasm -trimpath .
go run ../../ -wasm goroutine.wasm -namespace Go2DotNet.Example.Goroutine.AutoGen > gen.cs
dotnet run .
