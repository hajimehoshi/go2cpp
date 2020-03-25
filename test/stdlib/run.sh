set -e
echo "# Test $1"
env GOOS=js GOARCH=wasm go test -c -o test.wasm -trimpath $1
go run ../../cmd/gowasm2csharp -wasm test.wasm -namespace Go2DotNet.Test.StdLib.AutoGen
shift
dotnet run -c Release . -- $*
