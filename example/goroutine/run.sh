set -e
env GOOS=js GOARCH=wasm go build -tags example -o goroutine.wasm -trimpath .
go run ../../cmd/gowasm2csharp -wasm goroutine.wasm -namespace Go2DotNet.Example.Goroutine.AutoGen
dotnet run .
