set -e
env GOOS=js GOARCH=wasm go build -tags example -o monogame.wasm -trimpath .
rm -rf autogen
go run ../../cmd/gowasm2csharp -out autogen -wasm monogame.wasm -namespace Go2DotNet.Example.MonoGame.AutoGen
dotnet run .
