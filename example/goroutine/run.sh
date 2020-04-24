set -e
env GOOS=js GOARCH=wasm go build -tags example -o goroutine.wasm -trimpath .
rm -rf autogen
go run ../../cmd/gowasm2csharp -out autogen -wasm goroutine.wasm -namespace Go2DotNet.Example.Goroutine.AutoGen
dotnet run .
