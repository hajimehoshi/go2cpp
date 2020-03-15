env GOOS=js GOARCH=wasm go build -tags example -o monogame.wasm -trimpath .
go run ../../cmd/gowasm2csharp -wasm monogame.wasm -namespace Go2DotNet.Example.MonoGame.AutoGen > gen.cs
dotnet run .
